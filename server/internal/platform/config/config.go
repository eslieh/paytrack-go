// Package config loads process configuration from the environment. Each
// role only validates the settings it actually needs.
package config

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Role string

const (
	RoleAPI      Role = "api"      // tenant/dashboard REST API
	RoleCallback Role = "callback" // public Safaricom callback ingress
	RoleWorker   Role = "worker"   // outbox relay, consumers, reconciler, webhook dispatcher
	RoleMigrate  Role = "migrate"
	RoleAll      Role = "all" // every role in one process (local dev)
)

type MpesaConfig struct {
	Env            string // sandbox | production
	BaseURL        string
	ConsumerKey    string
	ConsumerSecret string
	ShortCode      string
	Passkey        string
}

type Config struct {
	DatabaseURL string

	APIAddr      string
	CallbackAddr string
	WorkerAddr   string // health endpoint for the worker role

	KafkaBrokers           []string
	KafkaPartitions        int
	KafkaReplicationFactor int

	EncryptionKey []byte
	JWTSecret     []byte

	// Dashboard sessions: short-lived access JWT + rotating refresh cookie.
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CookieSecure    bool // false only for plain-http local development
	APITrustXFF     bool // the API sits behind the Next.js proxy / a load balancer

	// AppBaseURL is the canonical dashboard URL used in email links.
	AppBaseURL string
	// APIPublicPath is where the dashboard proxies the API (Next.js rewrite),
	// e.g. "/api"; OAuth redirect URIs are <dashboard origin><path>/v1/auth/google/callback.
	APIPublicPath string

	GoogleClientID     string
	GoogleClientSecret string

	ResendAPIKey string
	EmailFrom    string
	EmailReplyTo string
	// SupportNotifyEmail overrides who gets support alerts (new tickets,
	// replies, ratings, feedback); empty means every active admin.
	SupportNotifyEmail string

	CORSAllowedOrigins []string

	// PublicCallbackBaseURL is the externally reachable base of the callback
	// role, e.g. https://cb.paytrack.example. Daraja is told to call
	// <base>/callbacks/mpesa/stk/<token>.
	PublicCallbackBaseURL string
	CallbackAllowedCIDRs  []*net.IPNet
	CallbackTrustXFF      bool

	// Platform Daraja app: signs every STK push and status query.
	Mpesa MpesaConfig

	WebhookAllowPrivate bool // allow webhook URLs that resolve to private IPs (dev only)

	// AdminAllowedCIDRs restricts the admin portal API (/admin/v1) to these
	// networks; empty allows any address.
	AdminAllowedCIDRs []*net.IPNet

	LogLevel slog.Level
}

func Load(role Role) (*Config, error) {
	c := &Config{
		DatabaseURL:           normalizeDatabaseURL(os.Getenv("DATABASE_URL")),
		APIAddr:               env("API_ADDR", ":9710"),
		CallbackAddr:          env("CALLBACK_ADDR", ":9711"),
		WorkerAddr:            env("WORKER_ADDR", ":9712"),
		KafkaBrokers:          list(os.Getenv("KAFKA_BROKERS")),
		CORSAllowedOrigins:    list(os.Getenv("CORS_ALLOWED_ORIGINS")),
		PublicCallbackBaseURL: strings.TrimRight(os.Getenv("PUBLIC_CALLBACK_BASE_URL"), "/"),
		Mpesa: MpesaConfig{
			Env:            env("MPESA_ENV", "sandbox"),
			ConsumerKey:    os.Getenv("MPESA_CONSUMER_KEY"),
			ConsumerSecret: os.Getenv("MPESA_CONSUMER_SECRET"),
			ShortCode:      os.Getenv("MPESA_SHORTCODE"),
			Passkey:        os.Getenv("MPESA_PASSKEY"),
		},
		CallbackTrustXFF:       os.Getenv("CALLBACK_TRUST_XFF") == "true",
		APITrustXFF:            os.Getenv("API_TRUST_XFF") == "true",
		CookieSecure:           os.Getenv("COOKIE_SECURE") != "false",
		AppBaseURL:             strings.TrimRight(os.Getenv("APP_BASE_URL"), "/"),
		APIPublicPath:          "/" + strings.Trim(env("API_PUBLIC_PATH", "/api"), "/"),
		GoogleClientID:         os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:     os.Getenv("GOOGLE_CLIENT_SECRET"),
		ResendAPIKey:           os.Getenv("RESEND_API_KEY"),
		EmailFrom:              env("EMAIL_FROM", "Paytrack <no-reply@localhost>"),
		EmailReplyTo:           os.Getenv("EMAIL_REPLY_TO"),
		SupportNotifyEmail:     os.Getenv("SUPPORT_NOTIFY_EMAIL"),
		WebhookAllowPrivate:    os.Getenv("WEBHOOK_ALLOW_PRIVATE") == "true",
		KafkaPartitions:        intEnv("KAFKA_PARTITIONS", 6),
		KafkaReplicationFactor: intEnv("KAFKA_REPLICATION_FACTOR", 1),
	}

	var missing []string
	need := func(name string, ok bool) {
		if !ok {
			missing = append(missing, name)
		}
	}
	need("DATABASE_URL", c.DatabaseURL != "")

	has := func(rs ...Role) bool {
		for _, r := range rs {
			if role == r || role == RoleAll {
				return true
			}
		}
		return false
	}

	if has(RoleAPI, RoleWorker) {
		key, err := base64.StdEncoding.DecodeString(os.Getenv("ENCRYPTION_KEY"))
		if err != nil || len(key) != 32 {
			missing = append(missing, "ENCRYPTION_KEY (base64 of 32 random bytes: openssl rand -base64 32)")
		}
		c.EncryptionKey = key
	}
	if has(RoleAPI, RoleWorker) {
		m := &c.Mpesa
		switch m.Env {
		case "sandbox":
			m.BaseURL = "https://sandbox.safaricom.co.ke"
		case "production":
			m.BaseURL = "https://api.safaricom.co.ke"
		default:
			missing = append(missing, `MPESA_ENV ("sandbox" or "production")`)
		}
		if u := os.Getenv("DARAJA_BASE_URL"); u != "" { // testing against a mock
			m.BaseURL = strings.TrimRight(u, "/")
		}
		need("MPESA_CONSUMER_KEY", m.ConsumerKey != "")
		need("MPESA_CONSUMER_SECRET", m.ConsumerSecret != "")
		need("MPESA_SHORTCODE", m.ShortCode != "")
		need("MPESA_PASSKEY", m.Passkey != "")
	}
	if has(RoleAPI) {
		c.JWTSecret = []byte(os.Getenv("JWT_SECRET"))
		need("JWT_SECRET (>= 32 chars)", len(c.JWTSecret) >= 32)
		var err error
		if c.AccessTokenTTL, err = time.ParseDuration(env("ACCESS_TOKEN_TTL", "15m")); err != nil {
			return nil, fmt.Errorf("ACCESS_TOKEN_TTL: %w", err)
		}
		if c.RefreshTokenTTL, err = time.ParseDuration(env("REFRESH_TOKEN_TTL", "720h")); err != nil {
			return nil, fmt.Errorf("REFRESH_TOKEN_TTL: %w", err)
		}
		if (c.GoogleClientID == "") != (c.GoogleClientSecret == "") {
			missing = append(missing, "GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET (set both, or neither to disable Google sign-in)")
		}
		need("PUBLIC_CALLBACK_BASE_URL", strings.HasPrefix(c.PublicCallbackBaseURL, "https://") || strings.HasPrefix(c.PublicCallbackBaseURL, "http://"))
	}
	if has(RoleWorker) {
		need("KAFKA_BROKERS", len(c.KafkaBrokers) > 0)
	}
	if has(RoleAPI, RoleWorker) {
		need("APP_BASE_URL (dashboard URL used in email links)", strings.HasPrefix(c.AppBaseURL, "http"))
	}
	if has(RoleCallback) {
		var err error
		if c.CallbackAllowedCIDRs, err = cidrs("CALLBACK_ALLOWED_CIDRS"); err != nil {
			return nil, err
		}
	}
	if has(RoleAPI) {
		var err error
		if c.AdminAllowedCIDRs, err = cidrs("ADMIN_ALLOWED_CIDRS"); err != nil {
			return nil, err
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing or invalid configuration: %s", strings.Join(missing, ", "))
	}

	if err := c.LogLevel.UnmarshalText([]byte(env("LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	return c, nil
}

func cidrs(name string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, s := range list(os.Getenv(name)) {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// normalizeDatabaseURL accepts SQLAlchemy-style URLs shared with Python
// services (postgresql+asyncpg://...) by dropping the "+driver" suffix.
func normalizeDatabaseURL(u string) string {
	scheme, rest, ok := strings.Cut(u, "://")
	if !ok {
		return u
	}
	if base, _, found := strings.Cut(scheme, "+"); found {
		scheme = base
	}
	return scheme + "://" + rest
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func intEnv(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil && n > 0 {
		return n
	}
	return def
}

func list(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
