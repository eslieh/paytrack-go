package main

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/eslieh/paytrack-go/server/api"
	adminhttp "github.com/eslieh/paytrack-go/server/internal/admin/adapters/http"
	"github.com/eslieh/paytrack-go/server/internal/admin/adapters/identitylink"
	adminpg "github.com/eslieh/paytrack-go/server/internal/admin/adapters/postgres"
	adminapp "github.com/eslieh/paytrack-go/server/internal/admin/app"
	callbackhttp "github.com/eslieh/paytrack-go/server/internal/callback/adapters/http"
	callbackpg "github.com/eslieh/paytrack-go/server/internal/callback/adapters/postgres"
	callbackapp "github.com/eslieh/paytrack-go/server/internal/callback/app"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
	googleadapter "github.com/eslieh/paytrack-go/server/internal/identity/adapters/google"
	identityhttp "github.com/eslieh/paytrack-go/server/internal/identity/adapters/http"
	identitypg "github.com/eslieh/paytrack-go/server/internal/identity/adapters/postgres"
	"github.com/eslieh/paytrack-go/server/internal/identity/adapters/security"
	identityapp "github.com/eslieh/paytrack-go/server/internal/identity/app"
	inboxhttp "github.com/eslieh/paytrack-go/server/internal/inbox/adapters/http"
	inboxkafka "github.com/eslieh/paytrack-go/server/internal/inbox/adapters/kafka"
	inboxpg "github.com/eslieh/paytrack-go/server/internal/inbox/adapters/postgres"
	inboxapp "github.com/eslieh/paytrack-go/server/internal/inbox/app"
	"github.com/eslieh/paytrack-go/server/internal/mail/adapters/logsender"
	mailpg "github.com/eslieh/paytrack-go/server/internal/mail/adapters/postgres"
	"github.com/eslieh/paytrack-go/server/internal/mail/adapters/resend"
	mailapp "github.com/eslieh/paytrack-go/server/internal/mail/app"
	"github.com/eslieh/paytrack-go/server/internal/mail/templates"
	notifyhttp "github.com/eslieh/paytrack-go/server/internal/notify/adapters/http"
	notifykafka "github.com/eslieh/paytrack-go/server/internal/notify/adapters/kafka"
	notifypg "github.com/eslieh/paytrack-go/server/internal/notify/adapters/postgres"
	notifytenant "github.com/eslieh/paytrack-go/server/internal/notify/adapters/tenantlink"
	"github.com/eslieh/paytrack-go/server/internal/notify/adapters/webhook"
	notifyapp "github.com/eslieh/paytrack-go/server/internal/notify/app"
	"github.com/eslieh/paytrack-go/server/internal/payment/adapters/daraja"
	paymenthttp "github.com/eslieh/paytrack-go/server/internal/payment/adapters/http"
	paymentkafka "github.com/eslieh/paytrack-go/server/internal/payment/adapters/kafka"
	paymentpg "github.com/eslieh/paytrack-go/server/internal/payment/adapters/postgres"
	paymenttenant "github.com/eslieh/paytrack-go/server/internal/payment/adapters/tenantlink"
	paymentapp "github.com/eslieh/paytrack-go/server/internal/payment/app"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/config"
	"github.com/eslieh/paytrack-go/server/internal/platform/crypto"
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
	"github.com/eslieh/paytrack-go/server/internal/platform/kafka"
	"github.com/eslieh/paytrack-go/server/internal/platform/outbox"
	"github.com/eslieh/paytrack-go/server/internal/platform/ratelimit"
	"github.com/eslieh/paytrack-go/server/internal/platform/run"
	"github.com/eslieh/paytrack-go/server/internal/support/adapters/accountlink"
	supporthttp "github.com/eslieh/paytrack-go/server/internal/support/adapters/http"
	supportpg "github.com/eslieh/paytrack-go/server/internal/support/adapters/postgres"
	supportapp "github.com/eslieh/paytrack-go/server/internal/support/app"
	tenanthttp "github.com/eslieh/paytrack-go/server/internal/tenant/adapters/http"
	tenantpg "github.com/eslieh/paytrack-go/server/internal/tenant/adapters/postgres"
	tenantapp "github.com/eslieh/paytrack-go/server/internal/tenant/app"
	"github.com/eslieh/paytrack-go/server/migrations"
)

// modules holds the application services; adapters are wired here and
// nowhere else.
type modules struct {
	identity *identityapp.Service
	tenants  *tenantapp.Service
	payments *paymentapp.Service
	notify   *notifyapp.Service
	callback *callbackapp.Service
	mail     *mailapp.Service
	admin    *adminapp.Service
	support  *supportapp.Service
	inbox    *inboxapp.Service
	limiter  *ratelimit.Limiter
}

// previewer adapts the mail module to the tenant module's Previewer port.
type previewer struct{ mail *mailapp.Service }

func (p previewer) Preview(template string, data map[string]any) (tenantapp.Rendered, error) {
	r, err := p.mail.Preview(template, data)
	return tenantapp.Rendered{Subject: r.Subject, HTML: r.HTML, Text: r.Text}, err
}

func build(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) (*modules, error) {
	m := &modules{callback: callbackapp.NewService(callbackpg.New(pool))}
	if cfg.EncryptionKey == nil { // callback-only process
		return m, nil
	}
	cipher, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	if err := crypto.VerifyKey(ctx, pool, cipher); err != nil {
		return nil, err
	}

	renderer, err := templates.New()
	if err != nil {
		return nil, err
	}
	var sender mailapp.Sender = logsender.Sender{}
	if cfg.ResendAPIKey != "" {
		sender = resend.New(cfg.ResendAPIKey)
	} else {
		slog.Warn("RESEND_API_KEY not set: emails are logged, not sent")
	}
	m.mail = mailapp.NewService(mailpg.New(pool), renderer, sender, cfg.EmailFrom, cfg.EmailReplyTo, cfg.AppBaseURL)
	m.limiter = ratelimit.New(pool)

	m.tenants = tenantapp.NewService(tenantpg.New(pool, cipher))

	var google identityapp.Google
	if cfg.GoogleClientID != "" {
		google = googleadapter.New(cfg.GoogleClientID, cfg.GoogleClientSecret)
	}
	codeKey := sha256.Sum256(append([]byte("paytrack-login-code|"), cfg.EncryptionKey...))
	m.identity, err = identityapp.NewService(identitypg.New(pool), security.Bcrypt{},
		security.JWT{Secret: cfg.JWTSecret, TTL: cfg.AccessTokenTTL}, m.mail, m.limiter, m.tenants, google,
		identityapp.Config{AppBaseURL: cfg.AppBaseURL, RefreshTTL: cfg.RefreshTokenTTL, CodeKey: codeKey[:], GoogleEnabled: google != nil})
	if err != nil {
		return nil, err
	}

	m.payments = paymentapp.NewService(paymentpg.New(pool), paymenttenant.Channels{Tenants: m.tenants},
		daraja.New(daraja.Credentials{
			BaseURL: cfg.Mpesa.BaseURL, ConsumerKey: cfg.Mpesa.ConsumerKey, ConsumerSecret: cfg.Mpesa.ConsumerSecret,
			ShortCode: cfg.Mpesa.ShortCode, Passkey: cfg.Mpesa.Passkey,
		}), cfg.PublicCallbackBaseURL)
	m.tenants.Wire(m.identity, m.payments, m.mail, previewer{m.mail}, cfg.AppBaseURL)
	m.notify = notifyapp.NewService(notifypg.New(pool), notifytenant.Targets{Tenants: m.tenants}, webhook.New(cfg.WebhookAllowPrivate),
		notifytenant.Prefs{Tenants: m.tenants}, m.mail, cfg.AppBaseURL)
	adminCodeKey := sha256.Sum256(append([]byte("paytrack-admin-code|"), cfg.EncryptionKey...))
	m.admin, err = adminapp.NewService(adminpg.New(pool), security.Bcrypt{}, m.mail, m.limiter, m.tenants,
		identitylink.Users{Identity: m.identity}, adminapp.Config{AppBaseURL: cfg.AppBaseURL, CodeKey: adminCodeKey[:]})
	if err != nil {
		return nil, err
	}
	m.support = supportapp.NewService(supportpg.New(pool), accountlink.Accounts{Identity: m.identity, Tenants: m.tenants},
		m.limiter, supportapp.Config{AppBaseURL: cfg.AppBaseURL, StaffEmail: cfg.SupportNotifyEmail})
	m.inbox = inboxapp.NewService(inboxpg.New(pool), m.mail)
	m.notify.Wire(m.inbox)
	return m, nil
}

func runRole(ctx context.Context, role config.Role, cfg *config.Config) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if role == config.RoleMigrate || role == config.RoleAll {
		if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
			return err
		}
		if role == config.RoleMigrate {
			return nil
		}
	}

	m, err := build(ctx, cfg, pool)
	if err != nil {
		return err
	}
	runsWorker := role == config.RoleWorker || role == config.RoleAll
	if runsWorker {
		topics := []string{contracts.TopicSTKCallbacks, contracts.TopicPaymentEvents, contracts.TopicNotifications}
		if err := kafka.EnsureTopics(ctx, cfg.KafkaBrokers, cfg.KafkaPartitions, cfg.KafkaReplicationFactor, topics...); err != nil {
			return err
		}
	}
	g, ctx := errgroup.WithContext(ctx)
	if role == config.RoleAPI || role == config.RoleAll {
		g.Go(func() error { return httpx.Serve(ctx, "api", cfg.APIAddr, apiHandler(cfg, m, pool)) })
	}
	if role == config.RoleCallback || role == config.RoleAll {
		g.Go(func() error { return httpx.Serve(ctx, "callback", cfg.CallbackAddr, callbackHandler(cfg, m, pool)) })
	}
	if runsWorker {
		startWorker(ctx, g, cfg, m, pool)
	}
	return g.Wait()
}

func health(mux *http.ServeMux, pool *pgxpool.Pool) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func apiHandler(cfg *config.Config, m *modules, pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	health(mux, pool)
	api.Register(mux, cfg.APITrustXFF)
	// API keys belong to tenants; everything else is a dashboard session.
	authenticate := auth.Middleware(auth.AuthenticatorFunc(func(ctx context.Context, bearer string) (auth.Principal, error) {
		if tenantapp.IsAPIKey(bearer) {
			return m.tenants.AuthenticateAPIKey(ctx, bearer)
		}
		return m.identity.Authenticate(ctx, bearer)
	}))
	// A suspended tenant's dashboard is read-only, but its users can still
	// sign out, manage their own account security and talk to support.
	readOnly := auth.ReadOnlyWhenSuspended(func(r *http.Request) bool {
		for _, prefix := range []string{"/v1/auth/", "/v1/me/", "/v1/support/", "/v1/notifications/"} {
			if strings.HasPrefix(r.URL.Path, prefix) {
				return true
			}
		}
		return false
	})
	authed := func(h http.Handler) http.Handler { return authenticate(readOnly(h)) }
	identityhttp.New(m.identity, m.tenants, identityhttp.Config{
		CookieSecure: cfg.CookieSecure, TrustXFF: cfg.APITrustXFF, AllowedOrigins: cfg.CORSAllowedOrigins,
		AppBaseURL: cfg.AppBaseURL, APIPublicPath: cfg.APIPublicPath,
	}).Register(mux, authed)
	samples := func(tpl string) map[string]any { return templates.Samples(cfg.AppBaseURL)[tpl] }
	tenanthttp.New(m.tenants, samples).Register(mux, authed)
	paymenthttp.New(m.payments).Register(mux, authed)
	notifyhttp.New(m.notify).Register(mux, authed)
	admin := adminhttp.New(m.admin, adminhttp.Config{
		CookieSecure: cfg.CookieSecure, TrustXFF: cfg.APITrustXFF, APIPublicPath: cfg.APIPublicPath,
		AllowedCIDRs: cfg.AdminAllowedCIDRs,
	})
	admin.Register(mux)
	supporthttp.New(m.support).Register(mux, authed, admin.Staff)
	inboxhttp.New(m.inbox).Register(mux, authed)
	return httpx.Chain(mux, httpx.Common, httpx.CORS(cfg.CORSAllowedOrigins))
}

func callbackHandler(cfg *config.Config, m *modules, pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	health(mux, pool)
	callbackhttp.New(m.callback, cfg.CallbackAllowedCIDRs, cfg.CallbackTrustXFF).Register(mux)
	return httpx.Chain(mux, httpx.Common)
}

func startWorker(ctx context.Context, g *errgroup.Group, cfg *config.Config, m *modules, pool *pgxpool.Pool) {
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	relay := &outbox.Relay{Pool: pool, Publisher: producer}

	g.Go(func() error {
		<-ctx.Done()
		return producer.Close()
	})
	g.Go(func() error {
		mux := http.NewServeMux()
		health(mux, pool)
		return httpx.Serve(ctx, "worker-health", cfg.WorkerAddr, mux)
	})
	g.Go(func() error { return run.Poll(ctx, "outbox-relay", 250*time.Millisecond, relay.RelayOnce) })
	g.Go(func() error {
		return run.Poll(ctx, "outbox-prune", time.Hour, func(ctx context.Context) (bool, error) {
			return false, relay.Prune(ctx, 7*24*time.Hour)
		})
	})
	g.Go((&kafka.Consumer{
		Brokers: cfg.KafkaBrokers, GroupID: "paytrack-payment", Topic: contracts.TopicSTKCallbacks,
		Handler: paymentkafka.CallbackHandler(m.payments), DLQ: producer,
	}).RunFunc(ctx))
	g.Go((&kafka.Consumer{
		Brokers: cfg.KafkaBrokers, GroupID: "paytrack-notify", Topic: contracts.TopicPaymentEvents,
		Handler: notifykafka.PaymentEventHandler(m.notify), DLQ: producer,
	}).RunFunc(ctx))
	g.Go((&kafka.Consumer{
		Brokers: cfg.KafkaBrokers, GroupID: "paytrack-inbox", Topic: contracts.TopicNotifications,
		Handler: inboxkafka.NotificationHandler(m.inbox), DLQ: producer,
	}).RunFunc(ctx))
	g.Go(func() error { return run.Poll(ctx, "reconciler", 5*time.Second, m.payments.Reconcile) })
	g.Go(func() error { return run.Poll(ctx, "webhook-dispatcher", time.Second, m.notify.Dispatch) })
	g.Go(func() error { return run.Poll(ctx, "mail-dispatcher", time.Second, m.mail.Dispatch) })
	g.Go(func() error { return run.Poll(ctx, "setup-reminders", time.Hour, m.tenants.SendSetupReminders) })
	g.Go(func() error {
		return run.Poll(ctx, "rate-limit-prune", time.Hour, func(ctx context.Context) (bool, error) {
			return false, m.limiter.Prune(ctx)
		})
	})
}
