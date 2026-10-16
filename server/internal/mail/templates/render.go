// Package templates renders transactional emails: an HTML version (shared
// layout in the dashboard's design language) and a plain-text version.
//
// Each template file defines "subject", "preheader", "content" (HTML) and
// "text"; it may override "footer".
package templates

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"github.com/eslieh/paytrack-go/server/internal/mail/app"
)

//go:embed *.html
var files embed.FS

var funcs = map[string]any{
	"dict": func(kv ...any) map[string]any {
		m := make(map[string]any, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			m[fmt.Sprint(kv[i])] = kv[i+1]
		}
		return m
	},
}

type Renderer struct {
	html map[string]*htmltemplate.Template
	text map[string]*texttemplate.Template
}

var _ app.Renderer = (*Renderer)(nil)

func New() (*Renderer, error) {
	layoutSrc, err := files.ReadFile("layout.html")
	if err != nil {
		return nil, err
	}
	layout := expand(string(layoutSrc))
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	r := &Renderer{html: map[string]*htmltemplate.Template{}, text: map[string]*texttemplate.Template{}}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".html")
		if name == "layout" || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		raw, err := files.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		src := expand(string(raw))
		h, err := htmltemplate.New("layout").Funcs(funcs).Parse(layout)
		if err != nil {
			return nil, fmt.Errorf("layout: %w", err)
		}
		if _, err := h.Parse(src); err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		t, err := texttemplate.New(name).Funcs(funcs).Parse(src)
		if err != nil {
			return nil, fmt.Errorf("template %s (text): %w", name, err)
		}
		r.html[name], r.text[name] = h, t
	}
	return r, nil
}

func (r *Renderer) Names() []string {
	out := make([]string, 0, len(r.html))
	for n := range r.html {
		out = append(out, n)
	}
	return out
}

func (r *Renderer) Render(name string, data map[string]any) (app.Rendered, error) {
	h, ok := r.html[name]
	if !ok {
		return app.Rendered{}, fmt.Errorf("unknown email template %q", name)
	}
	t := r.text[name]
	var subj, text, html bytes.Buffer
	if err := t.ExecuteTemplate(&subj, "subject", data); err != nil {
		return app.Rendered{}, fmt.Errorf("%s subject: %w", name, err)
	}
	if err := t.ExecuteTemplate(&text, "text", data); err != nil {
		return app.Rendered{}, fmt.Errorf("%s text: %w", name, err)
	}
	if err := h.ExecuteTemplate(&html, "layout", data); err != nil {
		return app.Rendered{}, fmt.Errorf("%s html: %w", name, err)
	}
	return app.Rendered{
		Subject: strings.TrimSpace(subj.String()),
		Text:    strings.TrimSpace(text.String()) + "\n",
		HTML:    html.String(),
	}, nil
}
