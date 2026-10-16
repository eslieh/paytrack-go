// Package kafka is the notify module's inbound adapter for payment events.
package kafka

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/notify/app"
	pkafka "github.com/eslieh/paytrack-go/server/internal/platform/kafka"
)

func PaymentEventHandler(svc *app.Service) pkafka.Handler {
	return func(ctx context.Context, _, value []byte) error {
		var ev contracts.PaymentEvent
		if err := json.Unmarshal(value, &ev); err != nil {
			return pkafka.Permanent(err)
		}
		if ev.ID == "" || ev.TenantID == "" {
			return pkafka.Permanent(errors.New("payment event missing id or tenant_id"))
		}
		if err := svc.Enqueue(ctx, ev); err != nil {
			return err
		}
		return svc.EmailOnPaymentEvent(ctx, ev)
	}
}
