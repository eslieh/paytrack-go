// Package webhook sends signed HTTP POSTs to tenant endpoints, refusing to
// connect to private/internal addresses unless explicitly allowed (SSRF
// protection: tenants control the URL).
package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/notify/app"
)

type Sender struct{ client *http.Client }

var _ app.Sender = (*Sender)(nil)

var errBlockedAddr = errors.New("webhook URL resolves to a private or reserved address")

func New(allowPrivate bool) *Sender {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !allowPrivate {
		// Control runs after DNS resolution, on the actual IP being dialled,
		// so DNS rebinding cannot bypass it.
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
				ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
				return errBlockedAddr
			}
			return nil
		}
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Sender{client: &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // a redirect is treated as a non-2xx response
		},
	}}
}

func (s *Sender) Send(ctx context.Context, url string, headers map[string]string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("posting webhook: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)) //nolint:errcheck // drain for connection reuse
	return resp.StatusCode, nil
}
