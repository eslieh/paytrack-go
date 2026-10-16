package app

import (
	"context"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/tenant/domain"
)

// ---------------------------------------------------------------- notification settings

func (s *Service) NotificationSettings(ctx context.Context, tenantID string) (domain.NotificationSettings, error) {
	return s.repo.GetNotificationSettings(ctx, tenantID)
}

type NotificationUpdate struct {
	PaymentReceipts      *bool
	ReceiptEmail         *string
	WebhookFailureAlerts *bool
}

func (s *Service) UpdateNotificationSettings(ctx context.Context, tenantID string, u NotificationUpdate) (domain.NotificationSettings, error) {
	cur, err := s.repo.GetNotificationSettings(ctx, tenantID)
	if err != nil {
		return cur, err
	}
	if u.PaymentReceipts != nil {
		cur.PaymentReceipts = *u.PaymentReceipts
	}
	if u.WebhookFailureAlerts != nil {
		cur.WebhookFailureAlerts = *u.WebhookFailureAlerts
	}
	if u.ReceiptEmail != nil {
		e := strings.ToLower(strings.TrimSpace(*u.ReceiptEmail))
		if e != "" {
			if a, err := mail.ParseAddress(e); err != nil || a.Address != e {
				return cur, errs.Invalid("invalid_receipt_email", "receipt_email must be a valid email address")
			}
		}
		cur.ReceiptEmail = e
	}
	cur.UpdatedAt = s.now()
	return cur, s.repo.SaveNotificationSettings(ctx, cur)
}

// ReceiptRecipient is where payment receipts and alerts go for a tenant.
func (s *Service) ReceiptRecipient(ctx context.Context, tenantID string) (string, error) {
	st, err := s.repo.GetNotificationSettings(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if st.ReceiptEmail != "" {
		return st.ReceiptEmail, nil
	}
	email, _, err := s.owners.OwnerContact(ctx, tenantID)
	return email, err
}

// PreviewEmail renders a notification email with sample data and the
// tenant's real name, for the settings page.
func (s *Service) PreviewEmail(ctx context.Context, tenantID, template string, sample map[string]any) (Rendered, error) {
	switch template {
	case "payment_receipt", "webhook_failing", "first_payment":
	default:
		return Rendered{}, errs.Invalid("invalid_template", "template must be payment_receipt, webhook_failing or first_payment")
	}
	name, err := s.TenantName(ctx, tenantID)
	if err != nil {
		return Rendered{}, err
	}
	sample["BusinessName"] = name
	return s.preview.Preview(template, sample)
}

// SendTestReceipt emails a sample receipt to the receipt recipient.
func (s *Service) SendTestReceipt(ctx context.Context, tenantID string, sample map[string]any) (string, error) {
	to, err := s.ReceiptRecipient(ctx, tenantID)
	if err != nil {
		return "", err
	}
	name, err := s.TenantName(ctx, tenantID)
	if err != nil {
		return "", err
	}
	sample["BusinessName"] = name
	return to, s.mailer.Enqueue(ctx, contracts.Email{Template: "payment_receipt", Category: contracts.EmailPayments, To: to, Data: sample})
}

// ---------------------------------------------------------------- onboarding

// Onboarding computes the setup checklist from real data.
func (s *Service) Onboarding(ctx context.Context, tenantID string) ([]domain.OnboardingStep, error) {
	_, verified, err := s.owners.OwnerContact(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	hasChannel, err := s.repo.HasActiveChannel(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	hasKey, err := s.repo.HasActiveAPIKey(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	paid, err := s.payments.HasSucceededPayment(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return []domain.OnboardingStep{
		{Key: "verify_email", Label: "Verify your email", Done: verified, Path: "/dashboard"},
		{Key: "channel", Label: "Add a channel (bank account, PayBill or Till)", Done: hasChannel, Path: "/dashboard/channels"},
		{Key: "api_key", Label: "Create an API key for your backend", Done: hasKey, Path: "/dashboard/api-keys"},
		{Key: "webhook", Label: "Set your webhook URL", Done: t.WebhookURL != "", Path: "/dashboard/webhooks"},
		{Key: "first_payment", Label: "Receive your first payment", Done: paid, Path: "/dashboard/payments"},
	}, nil
}

func complete(steps []domain.OnboardingStep) bool {
	for _, st := range steps {
		if !st.Done {
			return false
		}
	}
	return true
}

// SendSetupReminders emails owners who signed up 1 or 3 days ago and haven't
// finished setup. Dedupe keys make it safe to run hourly on every replica.
func (s *Service) SendSetupReminders(ctx context.Context) (bool, error) {
	now := s.now()
	tenants, err := s.repo.ListTenantsCreatedBetween(ctx, now.Add(-4*24*time.Hour), now.Add(-24*time.Hour))
	if err != nil {
		return false, err
	}
	for _, t := range tenants {
		day := 1
		if now.Sub(t.CreatedAt) >= 3*24*time.Hour {
			day = 3
		}
		email, verified, err := s.owners.OwnerContact(ctx, t.ID)
		if err != nil || !verified {
			continue // never send lifecycle email to an unproven address
		}
		steps, err := s.Onboarding(ctx, t.ID)
		if err != nil || complete(steps) {
			continue
		}
		view := make([]map[string]any, len(steps))
		for i, st := range steps {
			view[i] = map[string]any{"Label": st.Label, "Done": st.Done, "URL": s.appURL + st.Path}
		}
		if err := s.mailer.Enqueue(ctx, contracts.Email{
			Template: "setup_reminder", Category: contracts.EmailLifecycle, To: email,
			Data:      map[string]any{"BusinessName": t.Name, "Steps": view, "DashboardURL": s.appURL + "/dashboard"},
			DedupeKey: "setup_reminder:" + t.ID + ":" + string(rune('0'+day)),
		}); err != nil {
			slog.ErrorContext(ctx, "setup reminder failed", "tenant_id", t.ID, "err", err)
		}
	}
	return false, nil
}
