// Package domain holds the tenant module's entities and invariants.
package domain

import (
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

type Tenant struct {
	ID            string
	Name          string
	WebhookURL    string
	WebhookSecret string // plaintext in memory; encrypted at rest
	CreatedAt     time.Time
}

type Role string

const RoleOwner Role = "owner"

type User struct {
	ID           string
	TenantID     string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

// APIKey authenticates a tenant's backend. Only the SHA-256 hash is stored;
// the plaintext is shown once at creation.
type APIKey struct {
	ID         string
	TenantID   string
	Name       string
	Prefix     string // first characters of the key, for display
	Hash       []byte
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return "", errs.Invalid("invalid_email", "email address is invalid")
	}
	return s, nil
}

func ValidatePassword(pw string) error {
	if len(pw) < 10 || len(pw) > 72 { // bcrypt only uses the first 72 bytes
		return errs.Invalid("weak_password", "password must be 10-72 characters")
	}
	return nil
}

func ValidateName(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 100 {
		return "", errs.Invalid("invalid_"+field, field+" must be 1-100 characters")
	}
	return s, nil
}

// ValidateWebhookURL checks shape only. Resolution to private addresses is
// blocked at delivery time by the webhook sender.
func ValidateWebhookURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return "", errs.Invalid("invalid_webhook_url", "webhook url must be an absolute http(s) URL without credentials")
	}
	return u.String(), nil
}

// ---------------------------------------------------------------- channels

type ChannelType string

const (
	ChannelPaybill ChannelType = "paybill" // CustomerPayBillOnline
	ChannelTill    ChannelType = "till"    // CustomerBuyGoodsOnline
)

type Environment string

const (
	EnvSandbox    Environment = "sandbox"
	EnvProduction Environment = "production"
)

// Credentials are the tenant's Daraja app credentials for the channel.
type Credentials struct {
	ConsumerKey    string `json:"consumer_key"`
	ConsumerSecret string `json:"consumer_secret"`
	Passkey        string `json:"passkey"`
}

// Channel is one Daraja collection endpoint owned by a tenant: a PayBill,
// or a Till under a Buy Goods store number.
type Channel struct {
	ID          string
	TenantID    string
	Name        string
	Type        ChannelType
	Environment Environment
	// ShortCode is the BusinessShortCode used to sign STK requests: the
	// PayBill number, or the store (head office) number for a Till.
	ShortCode string
	// TillNumber is PartyB for Till channels; empty for PayBill.
	TillNumber  string
	Credentials Credentials
	Active      bool
	CreatedAt   time.Time
}

var shortCodeRe = regexp.MustCompile(`^\d{5,7}$`)

type NewChannelParams struct {
	Name        string
	Type        ChannelType
	Environment Environment
	ShortCode   string
	TillNumber  string
	Credentials Credentials
}

func NewChannel(id, tenantID string, p NewChannelParams, now time.Time) (*Channel, error) {
	name, err := ValidateName("name", p.Name)
	if err != nil {
		return nil, err
	}
	switch p.Type {
	case ChannelPaybill:
		p.TillNumber = ""
	case ChannelTill:
		if !shortCodeRe.MatchString(p.TillNumber) {
			return nil, errs.Invalid("invalid_till_number", "till_number must be 5-7 digits")
		}
	default:
		return nil, errs.Invalid("invalid_channel_type", `type must be "paybill" or "till"`)
	}
	if p.Environment != EnvSandbox && p.Environment != EnvProduction {
		return nil, errs.Invalid("invalid_environment", `environment must be "sandbox" or "production"`)
	}
	if !shortCodeRe.MatchString(p.ShortCode) {
		return nil, errs.Invalid("invalid_short_code", "short_code must be 5-7 digits")
	}
	c := p.Credentials
	c.ConsumerKey, c.ConsumerSecret, c.Passkey = strings.TrimSpace(c.ConsumerKey), strings.TrimSpace(c.ConsumerSecret), strings.TrimSpace(c.Passkey)
	if c.ConsumerKey == "" || c.ConsumerSecret == "" || c.Passkey == "" {
		return nil, errs.Invalid("missing_credentials", "consumer_key, consumer_secret and passkey are required")
	}
	return &Channel{
		ID: id, TenantID: tenantID, Name: name, Type: p.Type, Environment: p.Environment,
		ShortCode: p.ShortCode, TillNumber: p.TillNumber, Credentials: c, Active: true, CreatedAt: now,
	}, nil
}
