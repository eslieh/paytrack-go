// Package google implements the identity Google port: Google's OAuth 2.0
// authorization-code flow with PKCE, and ID-token verification against
// Google's signing keys.
package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/eslieh/paytrack-go/server/internal/identity/app"
)

const (
	authURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL = "https://oauth2.googleapis.com/token"
	certsURL = "https://www.googleapis.com/oauth2/v3/certs"
)

type Provider struct {
	clientID, clientSecret string
	verifier               *oidc.IDTokenVerifier
	http                   *http.Client
}

var _ app.Google = (*Provider)(nil)

func New(clientID, clientSecret string) *Provider {
	keys := oidc.NewRemoteKeySet(context.Background(), certsURL)
	return &Provider{
		clientID: clientID, clientSecret: clientSecret,
		// Google issues tokens as either "https://accounts.google.com" or
		// "accounts.google.com"; the issuer is checked manually below.
		verifier: oidc.NewVerifier("https://accounts.google.com", keys, &oidc.Config{ClientID: clientID, SkipIssuerCheck: true}),
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Provider) AuthCodeURL(state, nonce, challenge, redirectURI string) string {
	q := url.Values{
		"client_id": {p.clientID}, "redirect_uri": {redirectURI}, "response_type": {"code"},
		"scope": {"openid email profile"}, "state": {state}, "nonce": {nonce},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "prompt": {"select_account"},
	}
	return authURL + "?" + q.Encode()
}

func (p *Provider) Exchange(ctx context.Context, code, verifier, redirectURI, nonce string) (app.GoogleProfile, error) {
	form := url.Values{
		"code": {code}, "client_id": {p.clientID}, "client_secret": {p.clientSecret},
		"redirect_uri": {redirectURI}, "grant_type": {"authorization_code"}, "code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return app.GoogleProfile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.http.Do(req)
	if err != nil {
		return app.GoogleProfile{}, fmt.Errorf("google token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return app.GoogleProfile{}, fmt.Errorf("google token exchange: status %d: %s", resp.StatusCode, body)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		return app.GoogleProfile{}, errors.New("google token exchange: no id_token")
	}
	idt, err := p.verifier.Verify(ctx, tok.IDToken)
	if err != nil {
		return app.GoogleProfile{}, fmt.Errorf("google id_token: %w", err)
	}
	if idt.Issuer != "https://accounts.google.com" && idt.Issuer != "accounts.google.com" {
		return app.GoogleProfile{}, fmt.Errorf("google id_token: unexpected issuer %q", idt.Issuer)
	}
	if idt.Nonce != nonce {
		return app.GoogleProfile{}, errors.New("google id_token: nonce mismatch")
	}
	var c struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idt.Claims(&c); err != nil {
		return app.GoogleProfile{}, err
	}
	return app.GoogleProfile{Subject: idt.Subject, Email: c.Email, EmailVerified: c.EmailVerified, Name: c.Name}, nil
}
