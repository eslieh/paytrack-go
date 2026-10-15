// Package kafka is the payment module's inbound adapter for normalized STK
// callbacks published by the callback module.
package kafka

import (
	"context"
	"encoding/json"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/payment/app"
	pkafka "github.com/eslieh/paytrack-go/server/internal/platform/kafka"
)

func CallbackHandler(svc *app.Service) pkafka.Handler {
	return func(ctx context.Context, _, value []byte) error {
		var cb contracts.STKCallback
		if err := json.Unmarshal(value, &cb); err != nil {
			return pkafka.Permanent(err)
		}
		return svc.ApplyCallback(ctx, cb)
	}
}
