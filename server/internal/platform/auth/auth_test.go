package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOnlyWhenSuspended(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := ReadOnlyWhenSuspended(func(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/v1/me/") })(ok)

	cases := []struct {
		suspended    bool
		method, path string
		want         int
	}{
		{false, http.MethodPost, "/v1/payments", http.StatusNoContent},
		{true, http.MethodGet, "/v1/payments", http.StatusNoContent},
		{true, http.MethodPost, "/v1/payments", http.StatusForbidden},
		{true, http.MethodDelete, "/v1/channels/c1", http.StatusForbidden},
		{true, http.MethodPost, "/v1/me/password", http.StatusNoContent},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, c.path, nil)
		r = r.WithContext(WithPrincipal(r.Context(), Principal{Kind: KindUser, Suspended: c.suspended}))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("suspended=%v %s %s: got %d, want %d", c.suspended, c.method, c.path, w.Code, c.want)
		}
	}
}
