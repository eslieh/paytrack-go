// Package app holds the payment module's use cases.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/payment/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

type Service struct {
	repo            Repository
	channels        ChannelProvider
	gateway         Gateway
	callbackBaseURL string
	now             func() time.Time
}

func NewService(repo Repository, channels ChannelProvider, gateway Gateway, callbackBaseURL string) *Service {
	return &Service{repo: repo, channels: channels, gateway: gateway, callbackBaseURL: callbackBaseURL, now: time.Now}
}

type InitiateCmd struct {
	TenantID         string
	IdempotencyKey   string
	ChannelID        string
	Amount           int64
	Phone            string
	AccountReference string
	Description      string
	Metadata         map[string]string
}

func (c InitiateCmd) hash() string {
	b, _ := json.Marshal([]any{c.ChannelID, c.Amount, c.Phone, c.AccountReference, c.Description, c.Metadata})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Initiate creates a payment and sends the STK push. It is idempotent per
// (tenant, idempotency key): a retry with the same body returns the original
// payment; a retry with a different body is a conflict.
func (s *Service) Initiate(ctx context.Context, cmd InitiateCmd) (p *domain.Payment, replayed bool, err error) {
	if cmd.IdempotencyKey == "" || len(cmd.IdempotencyKey) > 255 {
		return nil, false, errs.Invalid("idempotency_key_required", "Idempotency-Key header is required (max 255 chars)")
	}
	reqHash := cmd.hash()
	if existing, err := s.replay(ctx, cmd, reqHash); existing != nil || err != nil {
		return existing, existing != nil, err
	}

	dest, err := s.channels.Resolve(ctx, cmd.TenantID, cmd.ChannelID)
	if errors.Is(err, ErrChannelNotFound) {
		return nil, false, errs.Invalid("channel_not_found", "channel_id does not refer to one of your channels")
	}
	if err != nil {
		return nil, false, err
	}
	if !dest.Active {
		return nil, false, errs.Invalid("channel_inactive", "channel is deactivated")
	}

	p, err = domain.New(domain.NewParams{
		ID: ids.New("pay"), TenantID: cmd.TenantID, Destination: dest,
		IdempotencyKey: cmd.IdempotencyKey, RequestHash: reqHash, CallbackToken: ids.Secret(32),
		Amount: cmd.Amount, Phone: cmd.Phone, AccountReference: cmd.AccountReference,
		Description: cmd.Description, Metadata: cmd.Metadata,
	}, s.now())
	if err != nil {
		return nil, false, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		if errors.Is(err, ErrDuplicateKey) { // lost a race with a concurrent retry
			existing, err := s.replay(ctx, cmd, reqHash)
			return existing, existing != nil, err
		}
		return nil, false, err
	}

	// The push must complete even if the client disconnects; otherwise we
	// could prompt the customer without recording the CheckoutRequestID.
	pushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 40*time.Second)
	defer cancel()
	ack, pushErr := s.gateway.STKPush(pushCtx, STKPushRequest{
		TransactionType: p.DestinationType.TransactionType(), PartyB: p.PartyB,
		Amount: p.Amount, Phone: p.Phone, AccountReference: p.AccountReference,
		Description: p.Description, CallbackURL: s.callbackURL(p.CallbackToken),
	})

	var rejected *RejectedError
	var notSent *NotSentError
	switch {
	case pushErr == nil:
		p, err = s.update(pushCtx, p, func(p *domain.Payment) []domain.Event {
			p.MarkPending(ack.MerchantRequestID, ack.CheckoutRequestID, s.now())
			return nil
		})
	case errors.As(pushErr, &rejected):
		p, err = s.update(pushCtx, p, func(p *domain.Payment) []domain.Event {
			p.MarkInitiationFailed(rejected.Message, s.now())
			return nil
		})
	case errors.As(pushErr, &notSent):
		slog.ErrorContext(ctx, "stk push not sent", "payment_id", p.ID, "err", pushErr)
		p, err = s.update(pushCtx, p, func(p *domain.Payment) []domain.Event {
			p.MarkInitiationFailed("M-Pesa is temporarily unavailable; please retry", s.now())
			return nil
		})
	default:
		// Outcome unknown: leave it in created. The callback (routed by
		// token) or the reconciler's timeout will settle it.
		slog.WarnContext(ctx, "stk push outcome unknown", "payment_id", p.ID, "err", pushErr)
	}
	return p, false, err
}

func (s *Service) replay(ctx context.Context, cmd InitiateCmd, reqHash string) (*domain.Payment, error) {
	existing, err := s.repo.GetByIdempotencyKey(ctx, cmd.TenantID, cmd.IdempotencyKey)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.RequestHash != reqHash {
		return nil, errs.Conflict("idempotency_key_reused", "Idempotency-Key was already used with a different request body")
	}
	return existing, nil
}

func (s *Service) callbackURL(token string) string {
	return s.callbackBaseURL + "/callbacks/mpesa/stk/" + token
}

// update applies mutate and saves, reloading and retrying on optimistic
// concurrency conflicts (e.g. a callback landing at the same moment).
func (s *Service) update(ctx context.Context, p *domain.Payment, mutate func(*domain.Payment) []domain.Event) (*domain.Payment, error) {
	for attempt := 0; ; attempt++ {
		before := p.UpdatedAt // every transition bumps UpdatedAt
		events := mutate(p)
		if p.UpdatedAt.Equal(before) && len(events) == 0 {
			return p, nil
		}
		err := s.repo.Save(ctx, p, events)
		if err == nil || !errors.Is(err, ErrVersionConflict) || attempt == 4 {
			return p, err
		}
		if p, err = s.repo.Get(ctx, p.TenantID, p.ID); err != nil {
			return nil, err
		}
	}
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (*domain.Payment, error) {
	p, err := s.repo.Get(ctx, tenantID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, errs.NotFound("payment_not_found", "payment not found")
	}
	return p, err
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]*domain.Payment, error) {
	if f.Status != "" {
		switch domain.Status(f.Status) {
		case domain.StatusCreated, domain.StatusPending, domain.StatusSucceeded, domain.StatusFailed, domain.StatusCancelled, domain.StatusExpired:
		default:
			return nil, errs.Invalid("invalid_status", "unknown status filter")
		}
	}
	if f.Phone != "" {
		phone, err := domain.NormalizePhone(f.Phone)
		if err != nil {
			return nil, err
		}
		f.Phone = phone
	}
	return s.repo.List(ctx, f)
}

// ApplyCallback settles a payment from a Daraja callback. Unknown tokens and
// mismatched checkout IDs are logged and dropped (returning nil), since
// retrying them cannot succeed.
func (s *Service) ApplyCallback(ctx context.Context, cb contracts.STKCallback) error {
	p, err := s.repo.GetByCallbackToken(ctx, cb.CallbackToken)
	if errors.Is(err, ErrNotFound) {
		slog.WarnContext(ctx, "callback for unknown token dropped", "inbox_id", cb.InboxID, "checkout_request_id", cb.CheckoutRequestID)
		return nil
	}
	if err != nil {
		return err
	}
	if p.CheckoutRequestID != "" && cb.CheckoutRequestID != p.CheckoutRequestID {
		slog.WarnContext(ctx, "callback checkout id mismatch dropped", "payment_id", p.ID, "inbox_id", cb.InboxID,
			"expected", p.CheckoutRequestID, "got", cb.CheckoutRequestID)
		return nil
	}
	if cb.ResultCode == domain.ResultSuccess && cb.Amount != 0 && cb.Amount != p.Amount {
		slog.ErrorContext(ctx, "callback amount differs from requested amount", "payment_id", p.ID, "requested", p.Amount, "paid", cb.Amount)
	}
	res := domain.Result{
		Code: cb.ResultCode, Desc: cb.ResultDesc, Receipt: cb.MpesaReceiptNumber,
		MerchantRequestID: cb.MerchantRequestID, CheckoutRequestID: cb.CheckoutRequestID,
	}
	_, err = s.update(ctx, p, func(p *domain.Payment) []domain.Event {
		return events(p.ApplyResult(res, s.now()))
	})
	return err
}

// Reconcile queries Daraja for payments whose callback is overdue. Safe to
// run on every worker replica concurrently.
func (s *Service) Reconcile(ctx context.Context) (more bool, err error) {
	const batch = 50
	due, err := s.repo.ClaimDueChecks(ctx, s.now(), batch, 2*time.Minute)
	if err != nil {
		return false, err
	}
	for _, p := range due {
		if err := s.reconcileOne(ctx, p); err != nil {
			slog.ErrorContext(ctx, "reconcile failed", "payment_id", p.ID, "err", err)
		}
	}
	return len(due) == batch, nil
}

func (s *Service) reconcileOne(ctx context.Context, p *domain.Payment) error {
	var res *domain.Result
	if p.CanQueryStatus() {
		qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		q, err := s.gateway.STKQuery(qctx, p.CheckoutRequestID)
		cancel()
		if err != nil {
			slog.WarnContext(ctx, "stk query failed", "payment_id", p.ID, "err", err)
		} else if !q.Pending {
			res = &q.Result
		}
	}
	_, err := s.update(ctx, p, func(p *domain.Payment) []domain.Event {
		if res != nil {
			return events(p.ApplyResult(*res, s.now()))
		}
		return events(p.RecordUnresolvedCheck(s.now()))
	})
	if errors.Is(err, ErrVersionConflict) {
		return nil // a callback settled it first
	}
	return err
}

func events(e *domain.Event) []domain.Event {
	if e == nil {
		return nil
	}
	return []domain.Event{*e}
}

// View maps a payment to its public representation (REST and webhooks).
func View(p *domain.Payment) contracts.PaymentView {
	return contracts.PaymentView{
		ID: p.ID, ChannelID: p.ChannelID, DestinationType: string(p.DestinationType), PartyB: p.PartyB, BankName: p.BankName,
		Status: string(p.Status), Amount: p.Amount, Currency: "KES",
		Phone: p.Phone, AccountReference: p.AccountReference, Description: p.Description, Metadata: p.Metadata,
		MerchantRequestID: p.MerchantRequestID, CheckoutRequestID: p.CheckoutRequestID,
		MpesaReceiptNumber: p.MpesaReceipt, ResultCode: p.ResultCode, ResultDesc: p.ResultDesc,
		FailureReason: p.FailureReason, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, CompletedAt: p.CompletedAt,
	}
}
