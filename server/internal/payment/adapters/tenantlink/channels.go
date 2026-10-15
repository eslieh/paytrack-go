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

func (c Channels) Resolve(ctx context.Context, tenantID, channelID string) (domain.Destination, error) {
	ch, err := c.Tenants.GetChannel(ctx, tenantID, channelID)
	if errs.IsKind(err, errs.KindNotFound) {
		return domain.Destination{}, app.ErrChannelNotFound
	}
	if err != nil {
		return domain.Destination{}, err
	}
	return domain.Destination{
		ChannelID: ch.ID, Type: domain.DestinationType(ch.Type), PartyB: ch.PartyB,
		BankName: ch.BankName, AccountNumber: ch.AccountNumber, Active: ch.Active,
	}, nil
}
