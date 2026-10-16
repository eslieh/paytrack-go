package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

func stringsLowerTrim(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

const (
	IntentSignin = "signin"
	IntentLink   = "link"
)

// GoogleStart begins the OAuth authorization-code flow (with PKCE). It
// returns the Google URL to redirect to and the state value the caller must
// store in a cookie to bind the flow to this browser.
func (s *Service) GoogleStart(ctx context.Context, intent, next, origin, redirectURI, userID string) (url, state string, err error) {
	if !s.cfg.GoogleEnabled {
		return "", "", errs.NotFound("google_disabled", "Google sign-in is not configured")
	}
	state = ids.SecretBase32(24)
	verifier := ids.SecretBase32(32)
	nonce := ids.SecretBase32(16)
	now := s.now()
	if err := s.repo.CreateToken(ctx, &domain.Token{
		ID: ids.New("tok"), UserID: userID, Purpose: domain.PurposeOAuthState, TokenHash: hashToken(state),
		Sends: 1, LastSentAt: now, CreatedAt: now, ExpiresAt: now.Add(domain.OAuthStateTTL),
		Data: map[string]string{"verifier": verifier, "nonce": nonce, "intent": intent, "next": next,
			"origin": origin, "redirect_uri": redirectURI},
	}); err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return s.google.AuthCodeURL(state, nonce, challenge, redirectURI), state, nil
}

// GoogleOutcome tells the HTTP adapter where to send the browser.
type GoogleOutcome struct {
	Origin      string
	Next        string
	Auth        *Auth  // signed in
	SignupToken string // new user: must choose a business name
	Linked      bool   // linked to the signed-in user
}

// GoogleCallback finishes the flow: validates state, exchanges the code and
// signs in, links, or starts sign-up.
func (s *Service) GoogleCallback(ctx context.Context, state, cookieState, code string, c Client) (*GoogleOutcome, error) {
	bad := errs.Invalid("google_state_invalid", "Google sign-in expired or was started in another browser; try again")
	if state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookieState)) != 1 {
		return nil, bad
	}
	t, err := s.repo.GetTokenByHash(ctx, domain.PurposeOAuthState, hashToken(state))
	if err != nil {
		return nil, bad
	}
	now := s.now()
	if t.Usable(now) != nil {
		return nil, bad
	}
	t.ConsumedAt = &now
	if err := s.repo.SaveToken(ctx, t); err != nil {
		return nil, err
	}
	out := &GoogleOutcome{Origin: t.Data["origin"], Next: t.Data["next"]}
	prof, err := s.google.Exchange(ctx, code, t.Data["verifier"], t.Data["redirect_uri"], t.Data["nonce"])
	if err != nil {
		return out, errs.Invalid("google_failed", "Google sign-in failed; try again")
	}
	if !prof.EmailVerified {
		return out, errs.Invalid("google_email_unverified", "your Google account's email is not verified")
	}
	prof.Email = stringsLowerTrim(prof.Email)

	if t.Data["intent"] == IntentLink {
		u, err := s.repo.GetUser(ctx, t.UserID)
		if err != nil {
			return out, err
		}
		if err := s.link(ctx, u, prof, c); err != nil {
			return out, err
		}
		out.Linked = true
		return out, nil
	}

	// Already linked: sign in.
	if ident, err := s.repo.GetIdentity(ctx, domain.ProviderGoogle, prof.Subject); err == nil {
		u, err := s.repo.GetUser(ctx, ident.UserID)
		if err != nil {
			return out, err
		}
		out.Auth, err = s.googleSignin(ctx, u, c)
		return out, err
	} else if !notFound(err) {
		return out, err
	}

	// Same email as an existing account: link it.
	if u, err := s.repo.GetUserByEmail(ctx, prof.Email); err == nil {
		if !u.Verified() {
			// Someone registered this address without proving they own it.
			// Google just proved ownership: take the account over safely by
			// dropping the unverified password and every session.
			u.PasswordHash, u.EmailVerifiedAt = "", &now
			u.UpdatedAt = now
			if err := s.repo.SaveUser(ctx, u); err != nil {
				return out, err
			}
			if err := s.signOutEverywhere(ctx, u, ""); err != nil {
				return out, err
			}
		}
		if err := s.link(ctx, u, prof, c); err != nil {
			return out, err
		}
		out.Auth, err = s.googleSignin(ctx, u, c)
		return out, err
	} else if !notFound(err) {
		return out, err
	}

	// New user: hold the Google profile until they name their business.
	raw := ids.SecretBase32(32)
	if err := s.repo.CreateToken(ctx, &domain.Token{
		ID: ids.New("tok"), Purpose: domain.PurposeGoogleSignup, TokenHash: hashToken(raw), Sends: 1,
		LastSentAt: now, CreatedAt: now, ExpiresAt: now.Add(domain.GoogleSignupTTL),
		Data: map[string]string{"sub": prof.Subject, "email": prof.Email, "name": prof.Name},
	}); err != nil {
		return out, err
	}
	out.SignupToken = raw
	return out, nil
}

func (s *Service) googleSignin(ctx context.Context, u *domain.User, c Client) (*Auth, error) {
	a, err := s.issue(ctx, u, domain.MethodGoogle, c)
	if err != nil {
		return nil, err
	}
	s.alertNewSignin(ctx, u, c, "Google", a.SessionID)
	return a, nil
}

func (s *Service) link(ctx context.Context, u *domain.User, prof GoogleProfile, c Client) error {
	if existing, err := s.repo.GetIdentity(ctx, domain.ProviderGoogle, prof.Subject); err == nil {
		if existing.UserID == u.ID {
			return nil
		}
		return errs.Conflict("google_in_use", "that Google account is already linked to another Paytrack account")
	}
	if err := s.repo.CreateIdentity(ctx, &domain.OAuthIdentity{
		Provider: domain.ProviderGoogle, Subject: prof.Subject, UserID: u.ID, Email: prof.Email, CreatedAt: s.now(),
	}); err != nil {
		return err
	}
	s.event(ctx, u, domain.EventGoogleLinked, c, map[string]string{"google_email": prof.Email})
	s.mail(ctx, u.Email, "google_linked", contracts.EmailSecurity, map[string]any{"GoogleEmail": prof.Email, "When": s.when()}, "")
	return nil
}

// GoogleSignupInfo returns the pending profile for the "name your business" step.
func (s *Service) GoogleSignupInfo(ctx context.Context, raw string) (email, name string, err error) {
	t, err := s.repo.GetTokenByHash(ctx, domain.PurposeGoogleSignup, hashToken(raw))
	if err != nil || t.Usable(s.now()) != nil {
		return "", "", errs.Invalid("invalid_token", "Google sign-up expired; start again")
	}
	return t.Data["email"], t.Data["name"], nil
}

// GoogleComplete creates the tenant and verified owner for a new Google user.
func (s *Service) GoogleComplete(ctx context.Context, raw, businessName string, c Client) (*Auth, error) {
	t, err := s.repo.GetTokenByHash(ctx, domain.PurposeGoogleSignup, hashToken(raw))
	if err != nil {
		return nil, errs.Invalid("invalid_token", "Google sign-up expired; start again")
	}
	now := s.now()
	if err := t.Usable(now); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(businessName)
	if name == "" || len(name) > 100 {
		return nil, errs.Invalid("invalid_business_name", "business_name must be 1-100 characters")
	}
	t.ConsumedAt = &now
	if err := s.repo.SaveToken(ctx, t); err != nil {
		return nil, err
	}
	u, err := s.createOwner(ctx, name, t.Data["email"], "", &now)
	if err != nil {
		if err == ErrEmailTaken {
			return nil, errs.Conflict("email_taken", "an account with this email already exists; sign in with Google again to link it")
		}
		return nil, err
	}
	if err := s.repo.CreateIdentity(ctx, &domain.OAuthIdentity{
		Provider: domain.ProviderGoogle, Subject: t.Data["sub"], UserID: u.ID, Email: u.Email, CreatedAt: now,
	}); err != nil {
		return nil, err
	}
	s.event(ctx, u, domain.EventSignup, c, map[string]string{"method": "google"})
	s.sendWelcome(ctx, u)
	return s.issue(ctx, u, domain.MethodGoogle, c)
}

// UnlinkGoogle removes Google sign-in; the account must keep a password.
func (s *Service) UnlinkGoogle(ctx context.Context, p auth.Principal, c Client) error {
	u, err := s.repo.GetUser(ctx, p.UserID)
	if err != nil {
		return err
	}
	if !u.HasPassword() {
		return errs.Invalid("password_required", "set a password before unlinking Google, or you'll be locked out")
	}
	if err := s.repo.DeleteIdentity(ctx, u.ID, domain.ProviderGoogle); notFound(err) {
		return errs.NotFound("not_linked", "Google is not linked")
	} else if err != nil {
		return err
	}
	s.event(ctx, u, domain.EventGoogleUnlinked, c, nil)
	return nil
}

var _ = time.Second
