// Package contracts defines the integration events exchanged between
// modules over Kafka. Modules depend on these types, never on each other's
// domain packages. Topic names carry a version; breaking changes get a new
// topic.
package contracts

import "time"

const (
	TopicSTKCallbacks  = "mpesa.stk-callbacks.v1"
	TopicPaymentEvents = "payment.events.v1"
)

// STKCallback is a normalized Daraja STK push result, published by the
// callback module and consumed by the payment module.
type STKCallback struct {
	InboxID            int64     `json:"inbox_id"`
	CallbackToken      string    `json:"callback_token"`
	MerchantRequestID  string    `json:"merchant_request_id"`
	CheckoutRequestID  string    `json:"checkout_request_id"`
	ResultCode         int       `json:"result_code"`
	ResultDesc         string    `json:"result_desc"`
	Amount             int64     `json:"amount,omitempty"`
	MpesaReceiptNumber string    `json:"mpesa_receipt_number,omitempty"`
	TransactionDate    string    `json:"transaction_date,omitempty"`
	PhoneNumber        string    `json:"phone_number,omitempty"`
	ReceivedAt         time.Time `json:"received_at"`
}

// Payment event types delivered to tenant webhooks.
const (
	EventPaymentSucceeded = "payment.succeeded"
	EventPaymentFailed    = "payment.failed"
	EventPaymentCancelled = "payment.cancelled"
	EventPaymentExpired   = "payment.expired"
)

// PaymentEvent is both the Kafka message and, verbatim, the webhook body
// sent to tenants.
type PaymentEvent struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	TenantID  string      `json:"tenant_id"`
	CreatedAt time.Time   `json:"created_at"`
	Data      PaymentView `json:"data"`
}

// PaymentView is the public representation of a payment, shared by the REST
// API and webhook payloads so tenants see one shape everywhere.
type PaymentView struct {
	ID                 string            `json:"id"`
	ChannelID          string            `json:"channel_id"`
	DestinationType    string            `json:"destination_type"`
	PartyB             string            `json:"business_number"`
	BankName           string            `json:"bank_name,omitempty"`
	Status             string            `json:"status"`
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	Phone              string            `json:"phone"`
	AccountReference   string            `json:"account_reference"`
	Description        string            `json:"description"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	MerchantRequestID  string            `json:"merchant_request_id,omitempty"`
	CheckoutRequestID  string            `json:"checkout_request_id,omitempty"`
	MpesaReceiptNumber string            `json:"mpesa_receipt_number,omitempty"`
	ResultCode         *int              `json:"result_code,omitempty"`
	ResultDesc         string            `json:"result_desc,omitempty"`
	FailureReason      string            `json:"failure_reason,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	CompletedAt        *time.Time        `json:"completed_at,omitempty"`
}
