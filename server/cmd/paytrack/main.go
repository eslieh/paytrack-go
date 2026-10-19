// Command paytrack runs one role of the PayTrack modular monolith. Every
// role is the same binary and image; deployments choose the role:
//
//	paytrack migrate   apply database migrations and exit
//	paytrack api       tenant/dashboard REST API
//	paytrack callback  public Safaricom callback ingress
//	paytrack worker    outbox relay, Kafka consumers, reconciler, webhook dispatcher
//	paytrack all       every role in one process (local development)
//
// and one admin command:
//
//	paytrack create-admin <email> [name]   create the admin portal account, or reset its password
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/eslieh/paytrack-go/server/internal/platform/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: paytrack <migrate|api|callback|worker|all>")
		os.Exit(2)
	}
	if os.Args[1] == "create-admin" {
		if err := createAdmin(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	role := config.Role(os.Args[1])
	switch role {
	case config.RoleAPI, config.RoleCallback, config.RoleWorker, config.RoleMigrate, config.RoleAll:
	default:
		fmt.Fprintf(os.Stderr, "unknown role %q\n", role)
		os.Exit(2)
	}

	cfg, err := config.Load(role)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})).With("role", string(role)))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runRole(ctx, role, cfg); err != nil {
		slog.Error("exiting", "err", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}
