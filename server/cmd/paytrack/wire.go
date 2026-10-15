package main

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	callbackhttp "github.com/eslieh/paytrack-go/server/internal/callback/adapters/http"
	callbackpg "github.com/eslieh/paytrack-go/server/internal/callback/adapters/postgres"
	callbackapp "github.com/eslieh/paytrack-go/server/internal/callback/app"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
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
	"github.com/eslieh/paytrack-go/server/internal/platform/run"
	tenanthttp "github.com/eslieh/paytrack-go/server/internal/tenant/adapters/http"
	tenantpg "github.com/eslieh/paytrack-go/server/internal/tenant/adapters/postgres"
	"github.com/eslieh/paytrack-go/server/internal/tenant/adapters/security"
	tenantapp "github.com/eslieh/paytrack-go/server/internal/tenant/app"
	"github.com/eslieh/paytrack-go/server/migrations"
)

// modules holds the application services; adapters are wired here and
// nowhere else.
type modules struct {
	tenants  *tenantapp.Service
	payments *paymentapp.Service
	notify   *notifyapp.Service
	callback *callbackapp.Service
}

func build(cfg *config.Config, pool *pgxpool.Pool) (*modules, error) {
	m := &modules{callback: callbackapp.NewService(callbackpg.New(pool))}
	if cfg.EncryptionKey == nil { // callback-only process
		return m, nil
	}
	cipher, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	m.tenants, err = tenantapp.NewService(tenantpg.New(pool, cipher), security.Bcrypt{},
		security.JWT{Secret: cfg.JWTSecret, TTL: cfg.JWTTTL})
	if err != nil {
		return nil, err
	}
	m.payments = paymentapp.NewService(paymentpg.New(pool), paymenttenant.Channels{Tenants: m.tenants},
		daraja.New(cfg.DarajaSandboxURL, cfg.DarajaProductionURL), cfg.PublicCallbackBaseURL)
	m.notify = notifyapp.NewService(notifypg.New(pool), notifytenant.Targets{Tenants: m.tenants}, webhook.New(cfg.WebhookAllowPrivate))
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

	m, err := build(cfg, pool)
	if err != nil {
		return err
	}
	runsWorker := role == config.RoleWorker || role == config.RoleAll
	if runsWorker {
		topics := []string{contracts.TopicSTKCallbacks, contracts.TopicPaymentEvents}
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
	authed := auth.Middleware(m.tenants)
	tenanthttp.New(m.tenants).Register(mux, authed)
	paymenthttp.New(m.payments).Register(mux, authed)
	notifyhttp.New(m.notify).Register(mux, authed)
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
	g.Go(func() error { return run.Poll(ctx, "reconciler", 5*time.Second, m.payments.Reconcile) })
	g.Go(func() error { return run.Poll(ctx, "webhook-dispatcher", time.Second, m.notify.Dispatch) })
}
