// Package kafka is the inbox module's inbound adapter for notifications.
package kafka

import (
	"context"
	"encoding/json"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/inbox/app"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	pkafka "github.com/eslieh/paytrack-go/server/internal/platform/kafka"
)

func NotificationHandler(svc *app.Service) pkafka.Handler {
	return func(ctx context.Context, _, value []byte) error {
		var n contracts.Notification
		if err := json.Unmarshal(value, &n); err != nil {
			return pkafka.Permanent(err)
		}
		err := svc.Deliver(ctx, n)
		if errs.IsKind(err, errs.KindInvalid) {
			return pkafka.Permanent(err)
		}
		return err
	}
}
