package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServesSpecAsJSON(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, true)
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	req.Host = "internal:9710"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "api.paytrack.example")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var doc struct {
		OpenAPI string              `json:"openapi"`
		Paths   map[string]any      `json:"paths"`
		Servers []map[string]string `json:"servers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OpenAPI == "" || len(doc.Paths) < 10 || doc.Servers[0]["url"] != "https://api.paytrack.example" {
		t.Fatalf("unexpected spec: openapi=%q paths=%d servers=%v", doc.OpenAPI, len(doc.Paths), doc.Servers)
	}
	if _, ok := doc.Paths["/v1/payments"]; !ok {
		t.Fatal("missing /v1/payments")
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/openapi.yaml", nil))
	if rec.Code != 200 || rec.Body.Len() == 0 {
		t.Fatalf("yaml: status %d", rec.Code)
	}
}
