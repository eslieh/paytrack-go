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
