// Package logsender "sends" email by logging it. Used in development when
// RESEND_API_KEY is not set, so verification links and login codes are
// visible in the logs.
package logsender

import (
	"context"
	"log/slog"

	"github.com/eslieh/paytrack-go/server/internal/mail/app"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

type Sender struct{}

var _ app.Sender = Sender{}

func (Sender) Send(_ context.Context, m app.Outgoing) (string, error) {
	slog.Info("email (log sender)", "to", m.To, "subject", m.Subject, "text", m.Text)
	return ids.New("log"), nil
}
