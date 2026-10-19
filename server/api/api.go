// Package api embeds the OpenAPI contract and serves it as JSON or YAML.
package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var specYAML []byte

var (
	specOnce sync.Once
	spec     map[string]any
	specErr  error
)

func load() (map[string]any, error) {
	specOnce.Do(func() {
		specErr = yaml.Unmarshal(specYAML, &spec)
		if specErr == nil {
			_, specErr = json.Marshal(spec) // fail early on YAML that has no JSON form
		}
	})
	return spec, specErr
}

// Register mounts GET /openapi.json and GET /openapi.yaml. The servers list
// is set to the origin the spec was fetched from, so tools that import it
// (Postman, Swagger UI, code generators) call the right API.
func Register(mux *http.ServeMux, trustXFF bool) {
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		s, err := load()
		if err != nil {
			http.Error(w, "spec unavailable", http.StatusInternalServerError)
			return
		}
		out := make(map[string]any, len(s))
		for k, v := range s {
			out[k] = v
		}
		out["servers"] = []map[string]string{{"url": origin(r, trustXFF)}}
		public(w)
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	})
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		public(w)
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(specYAML)
	})
}

// public marks the spec as cacheable and readable from any site (Swagger UI,
// editors); it contains no secrets and no credentials are involved.
func public(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func origin(r *http.Request, trustXFF bool) string {
	scheme, host := "http", r.Host
	if r.TLS != nil {
		scheme = "https"
	}
	if trustXFF {
		if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
			scheme, _, _ = strings.Cut(p, ",")
		}
		if h := r.Header.Get("X-Forwarded-Host"); h != "" {
			host, _, _ = strings.Cut(h, ",")
		}
	}
	return fmt.Sprintf("%s://%s", strings.TrimSpace(scheme), strings.TrimSpace(host))
}
