package http

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	_, office, _ := net.ParseCIDR("203.0.113.0/24")

	cases := []struct {
		name     string
		cidrs    []*net.IPNet
		method   string
		remote   string
		header   bool
		wantCode int
	}{
		{"get without header", nil, http.MethodGet, "198.51.100.1:1", false, http.StatusNoContent},
		{"post without header", nil, http.MethodPost, "198.51.100.1:1", false, http.StatusForbidden},
		{"post with header", nil, http.MethodPost, "198.51.100.1:1", true, http.StatusNoContent},
		{"outside allowlist", []*net.IPNet{office}, http.MethodGet, "198.51.100.1:1", true, http.StatusNotFound},
		{"inside allowlist", []*net.IPNet{office}, http.MethodGet, "203.0.113.7:1", true, http.StatusNoContent},
	}
	for _, c := range cases {
		h := (&Handler{cfg: Config{AllowedCIDRs: c.cidrs}}).guard(ok)
		r := httptest.NewRequest(c.method, "/admin/v1/me", nil)
		r.RemoteAddr = c.remote
		if c.header {
			r.Header.Set(csrfHeader, "1")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.wantCode {
			t.Errorf("%s: got %d, want %d", c.name, w.Code, c.wantCode)
		}
	}
}
