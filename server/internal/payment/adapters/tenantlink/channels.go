// Package tenantlink adapts the tenant module (in-process) to the payment
// module's ChannelProvider port. If tenants become a separate service, only
// this adapter changes (to a gRPC/HTTP client).
package tenantlink

import (
	"context"

	"github.com/eslieh/paytrack-go/server/internal/payment/app"
	"github.com/eslieh/paytrack-go/server/internal/payment/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	tenantapp "github.com/eslieh/paytrack-go/server/internal/tenant/app"
)

type Channels struct{ Tenants *tenantapp.Service }

var _ app.ChannelProvider = Channels{}

func (c Channels) Resolve(ctx context.Context, tenantID, channelID string) (domain.Channel, error) {
	ch, err := c.Tenants.GetChannel(ctx, tenantID, channelID)
	if errs.IsKind(err, errs.KindNotFound) {
		return domain.Channel{}, app.ErrChannelNotFound
	}
	if err != nil {
		return domain.Channel{}, err
	}
	return domain.Channel{
		ID: ch.ID, TenantID: ch.TenantID, Type: string(ch.Type), Environment: string(ch.Environment),
		ShortCode: ch.ShortCode, TillNumber: ch.TillNumber, Active: ch.Active,
		ConsumerKey: ch.Credentials.ConsumerKey, ConsumerSecret: ch.Credentials.ConsumerSecret, Passkey: ch.Credentials.Passkey,
	}, nil
}
