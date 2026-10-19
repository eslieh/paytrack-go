// Package identitylink adapts the identity module (in-process) to the admin
// module's Users port.
package identitylink

import (
	"context"

	"github.com/eslieh/paytrack-go/server/internal/admin/app"
	identityapp "github.com/eslieh/paytrack-go/server/internal/identity/app"
)

type Users struct{ Identity *identityapp.Service }

var _ app.Users = Users{}

func (u Users) OwnerContact(ctx context.Context, tenantID string) (string, bool, error) {
	return u.Identity.OwnerContact(ctx, tenantID)
}

func (u Users) UserContact(ctx context.Context, id string) (string, string, error) {
	return u.Identity.UserContact(ctx, id)
}

func (u Users) DisableUser(ctx context.Context, id, reason string, c app.Client) error {
	return u.Identity.DisableUser(ctx, id, reason, identityapp.Client(c))
}

func (u Users) EnableUser(ctx context.Context, id string, c app.Client) error {
	return u.Identity.EnableUser(ctx, id, identityapp.Client(c))
}

func (u Users) SendPasswordReset(ctx context.Context, id string) error {
	return u.Identity.SendPasswordReset(ctx, id)
}

func (u Users) ResendVerification(ctx context.Context, id string) error {
	return u.Identity.ResendVerificationFor(ctx, id)
}

func (u Users) RevokeAllSessions(ctx context.Context, id string, c app.Client) error {
	return u.Identity.RevokeAllSessions(ctx, id, identityapp.Client(c))
}

func (u Users) ResetTwoStep(ctx context.Context, id string, c app.Client) error {
	return u.Identity.ResetTwoStep(ctx, id, identityapp.Client(c))
}

func (u Users) UnlinkGoogle(ctx context.Context, id string, c app.Client) error {
	return u.Identity.UnlinkGoogleFor(ctx, id, identityapp.Client(c))
}

func (u Users) ClearSignInLimits(ctx context.Context, id string) error {
	return u.Identity.ClearSignInLimits(ctx, id)
}

func (u Users) ChangeEmail(ctx context.Context, id, email string, c app.Client) (string, error) {
	return u.Identity.ChangeEmail(ctx, id, email, identityapp.Client(c))
}
