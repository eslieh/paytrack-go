// Package accountlink adapts the identity and tenant modules (in-process) to
// the support module's Accounts port.
package accountlink

import (
	"context"

	identityapp "github.com/eslieh/paytrack-go/server/internal/identity/app"
	"github.com/eslieh/paytrack-go/server/internal/support/app"
	tenantapp "github.com/eslieh/paytrack-go/server/internal/tenant/app"
)

type Accounts struct {
	Identity *identityapp.Service
	Tenants  *tenantapp.Service
}

var _ app.Accounts = Accounts{}

func (a Accounts) UserContact(ctx context.Context, userID string) (string, string, error) {
	return a.Identity.UserContact(ctx, userID)
}

func (a Accounts) TenantName(ctx context.Context, tenantID string) (string, error) {
	return a.Tenants.TenantName(ctx, tenantID)
}
