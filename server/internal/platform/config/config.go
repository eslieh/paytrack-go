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
	JWTTTL        time.Duration

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
		ttl, err := time.ParseDuration(env("JWT_TTL", "12h"))
		if err != nil {
			return nil, fmt.Errorf("JWT_TTL: %w", err)
		}
		c.JWTTTL = ttl
		need("PUBLIC_CALLBACK_BASE_URL", strings.HasPrefix(c.PublicCallbackBaseURL, "https://") || strings.HasPrefix(c.PublicCallbackBaseURL, "http://"))
	}
	if has(RoleWorker) {
		need("KAFKA_BROKERS", len(c.KafkaBrokers) > 0)
	}
	if has(RoleCallback) {
		for _, s := range list(os.Getenv("CALLBACK_ALLOWED_CIDRS")) {
			_, n, err := net.ParseCIDR(s)
			if err != nil {
				return nil, fmt.Errorf("CALLBACK_ALLOWED_CIDRS: %w", err)
			}
			c.CallbackAllowedCIDRs = append(c.CallbackAllowedCIDRs, n)
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
