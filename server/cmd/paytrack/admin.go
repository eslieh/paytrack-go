package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	adminpg "github.com/eslieh/paytrack-go/server/internal/admin/adapters/postgres"
	adminapp "github.com/eslieh/paytrack-go/server/internal/admin/app"
	"github.com/eslieh/paytrack-go/server/internal/identity/adapters/security"
	"github.com/eslieh/paytrack-go/server/internal/platform/config"
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
)

// createAdmin creates the platform admin account, or resets its password
// (and signs out its sessions) if it exists:
//
//	ADMIN_PASSWORD=... paytrack create-admin ops@paytrack.example "Ops"
//	paytrack create-admin ops@paytrack.example < password.txt
func createAdmin(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: paytrack create-admin <email> [name]")
	}
	email, name := args[0], strings.Join(args[1:], " ")
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		fmt.Fprint(os.Stderr, "Password (12-72 characters): ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("read password: %w", err)
		}
		password = strings.TrimRight(line, "\r\n")
	}

	cfg, err := config.Load(config.RoleMigrate) // only needs DATABASE_URL
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	// Sign-in only; no mailer, limiter or other modules are needed here.
	svc, err := adminapp.NewService(adminpg.New(pool), security.Bcrypt{}, nil, nil, nil, nil, adminapp.Config{})
	if err != nil {
		return err
	}
	created, err := svc.EnsureStaff(ctx, email, name, password)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(os.Stderr, "created admin %s\n", email)
	} else {
		fmt.Fprintf(os.Stderr, "reset password for admin %s and signed out its sessions\n", email)
	}
	return nil
}
