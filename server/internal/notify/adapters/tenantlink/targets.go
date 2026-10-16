// Package tenantlink adapts the tenant module to the notify module's
// TargetProvider port.
package tenantlink

import (
	"context"

	"github.com/eslieh/paytrack-go/server/internal/notify/app"
	tenantapp "github.com/eslieh/paytrack-go/server/internal/tenant/app"
)

type Targets struct{ Tenants *tenantapp.Service }

var _ app.TargetProvider = Targets{}

func (t Targets) Target(ctx context.Context, tenantID string) (app.Target, error) {
	wh, err := t.Tenants.GetWebhook(ctx, tenantID)
	if err != nil {
		return app.Target{}, err
	}
	return app.Target{URL: wh.URL, Secret: wh.Secret}, nil
}

// Prefs implements app.PrefsProvider from the tenant module.
type Prefs struct{ Tenants *tenantapp.Service }

var _ app.PrefsProvider = Prefs{}

func (p Prefs) Prefs(ctx context.Context, tenantID string) (app.Prefs, error) {
	st, err := p.Tenants.NotificationSettings(ctx, tenantID)
	if err != nil {
		return app.Prefs{}, err
	}
	name, err := p.Tenants.TenantName(ctx, tenantID)
	if err != nil {
		return app.Prefs{}, err
	}
	to, err := p.Tenants.ReceiptRecipient(ctx, tenantID)
	if err != nil {
		return app.Prefs{}, err
	}
	return app.Prefs{BusinessName: name, Recipient: to, PaymentReceipts: st.PaymentReceipts,
		WebhookFailureAlerts: st.WebhookFailureAlerts}, nil
}
