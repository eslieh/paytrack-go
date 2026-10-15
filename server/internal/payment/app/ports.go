package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/payment/domain"
)

var (
	ErrNotFound        = errors.New("payment not found")
	ErrDuplicateKey    = errors.New("duplicate idempotency key")
	ErrVersionConflict = errors.New("payment was modified concurrently")
	ErrChannelNotFound = errors.New("channel not found")
)

// Repository persists payments. Save must write the given events to the
// outbox in the same transaction as the state change, and fail with
// ErrVersionConflict if p.Version no longer matches.
type Repository interface {
	Create(ctx context.Context, p *domain.Payment) error
	Get(ctx context.Context, tenantID, id string) (*domain.Payment, error)
	GetByIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.Payment, error)
	GetByCallbackToken(ctx context.Context, token string) (*domain.Payment, error)
	Save(ctx context.Context, p *domain.Payment, events []domain.Event) error
	List(ctx context.Context, f ListFilter) ([]*domain.Payment, error)
	// ClaimDueChecks returns payments whose NextCheckAt has passed and pushes
	// their NextCheckAt forward by lease, so concurrent reconcilers on other
	// replicas skip them.
	ClaimDueChecks(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]*domain.Payment, error)
}

type ListFilter struct {
	TenantID  string
	Status    string
	ChannelID string
	Phone     string
	Before    string // cursor: return payments with id < Before
	Limit     int
}

// ChannelProvider resolves a tenant's channel, credentials included.
type ChannelProvider interface {
	Resolve(ctx context.Context, tenantID, channelID string) (domain.Channel, error)
}

type STKPushRequest struct {
	Amount           int64
	Phone            string
	AccountReference string
	Description      string
	CallbackURL      string
}

type STKPushAck struct {
	MerchantRequestID string
	CheckoutRequestID string
	CustomerMessage   string
}

type STKQueryResult struct {
	Pending bool // Daraja is still processing; ask again later
	Result  domain.Result
}

// Gateway is the outbound port to M-Pesa.
type Gateway interface {
	STKPush(ctx context.Context, ch domain.Channel, req STKPushRequest) (STKPushAck, error)
	STKQuery(ctx context.Context, ch domain.Channel, checkoutRequestID string) (STKQueryResult, error)
}

// RejectedError is returned by Gateway.STKPush when Daraja definitively
// refused the request (bad credentials, invalid shortcode, ...). Any other
// error means the outcome is unknown and the push may have gone through.
type RejectedError struct {
	Code    string
	Message string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("daraja rejected request: %s %s", e.Code, e.Message)
}
