package main

import (
	"fmt"
	"os"
)

type Config struct {
	ConsumerKey    string
	ConsumerSecret string
	ShortCode      string
	Passkey        string
	CallbackURL    string
	BaseURL        string // sandbox or production Daraja base URL
	ListenAddr     string
}

func loadConfig() (*Config, error) {
	env := os.Getenv("MPESA_ENV")
	baseURL := "https://sandbox.safaricom.co.ke"
	if env == "production" {
		baseURL = "https://api.safaricom.co.ke"
	}

	cfg := &Config{
		ConsumerKey:    os.Getenv("MPESA_CONSUMER_KEY"),
		ConsumerSecret: os.Getenv("MPESA_CONSUMER_SECRET"),
		ShortCode:      os.Getenv("MPESA_SHORTCODE"),
		Passkey:        os.Getenv("MPESA_PASSKEY"),
		CallbackURL:    os.Getenv("MPESA_CALLBACK_URL"),
		BaseURL:        baseURL,
		ListenAddr:     os.Getenv("LISTEN_ADDR"),
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}

	missing := []string{}
	if cfg.ConsumerKey == "" {
		missing = append(missing, "MPESA_CONSUMER_KEY")
	}
	if cfg.ConsumerSecret == "" {
		missing = append(missing, "MPESA_CONSUMER_SECRET")
	}
	if cfg.ShortCode == "" {
		missing = append(missing, "MPESA_SHORTCODE")
	}
	if cfg.Passkey == "" {
		missing = append(missing, "MPESA_PASSKEY")
	}
	if cfg.CallbackURL == "" {
		missing = append(missing, "MPESA_CALLBACK_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}
