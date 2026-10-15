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

// ChannelType selects how the platform routes an STK push.
type ChannelType string

const (
	ChannelBank    ChannelType = "bank"    // bank's PayBill as PartyB, bank account number as AccountReference
	ChannelPaybill ChannelType = "paybill" // any PayBill as PartyB, account number as AccountReference
	ChannelTill    ChannelType = "till"    // Till as PartyB (Buy Goods)
)

// Channel is a tenant's registered payment destination. STK pushes are
// signed with the platform's Daraja app; the channel only decides where the
// money is routed (PartyB and AccountReference).
type Channel struct {
	ID       string
	TenantID string
	Name     string
	Type     ChannelType
	// PartyB is the receiving shortcode: the bank's PayBill, the PayBill, or the Till.
	PartyB   string
	BankName string // bank channels only
	// AccountNumber is the bank account number (required for bank), or the
	// default PayBill account (optional). Unused for Till.
	AccountNumber string
	Active        bool
	CreatedAt     time.Time
}

var (
	shortCodeRe  = regexp.MustCompile(`^\d{5,7}$`)
	accountRefRe = regexp.MustCompile(`^[A-Za-z0-9\-_/. ]{1,12}$`) // Daraja AccountReference limit
)

type NewChannelParams struct {
	Name           string
	Type           ChannelType
	BankCode       string // bank: the bank's PayBill shortcode, from Banks
	BusinessNumber string // paybill/till: the PayBill or Till number
	AccountNumber  string
}

func NewChannel(id, tenantID string, p NewChannelParams, now time.Time) (*Channel, error) {
	name, err := ValidateName("name", p.Name)
	if err != nil {
		return nil, err
	}
	c := &Channel{ID: id, TenantID: tenantID, Name: name, Type: p.Type, Active: true, CreatedAt: now}
	account := strings.TrimSpace(p.AccountNumber)
	if account != "" && !accountRefRe.MatchString(account) {
		return nil, errs.Invalid("invalid_account_number", "account_number must be 1-12 characters: letters, digits, space and -_/.")
	}
	switch p.Type {
	case ChannelBank:
		bank, ok := BankByCode(p.BankCode)
		if !ok {
			return nil, errs.Invalid("invalid_bank_code", "bank_code must be one of the codes from GET /v1/banks")
		}
		if account == "" {
			return nil, errs.Invalid("account_number_required", "account_number (the bank account to credit) is required for bank channels")
		}
		c.PartyB, c.BankName, c.AccountNumber = bank.Code, bank.Name, account
	case ChannelPaybill:
		if !shortCodeRe.MatchString(p.BusinessNumber) {
			return nil, errs.Invalid("invalid_business_number", "business_number must be a 5-7 digit PayBill number")
		}
		c.PartyB, c.AccountNumber = p.BusinessNumber, account
	case ChannelTill:
		if !shortCodeRe.MatchString(p.BusinessNumber) {
			return nil, errs.Invalid("invalid_business_number", "business_number must be a 5-7 digit Till number")
		}
		c.PartyB = p.BusinessNumber
	default:
		return nil, errs.Invalid("invalid_channel_type", `type must be "bank", "paybill" or "till"`)
	}
	return c, nil
}
