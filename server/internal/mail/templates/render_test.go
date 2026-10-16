package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryTemplateRendersWithSamples(t *testing.T) {
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	samples := Samples("https://paytrack.example")
	if len(r.Names()) != len(samples) {
		t.Fatalf("%d templates but %d samples", len(r.Names()), len(samples))
	}
	out := os.Getenv("EMAIL_PREVIEW_DIR") // set to write HTML files for a visual check
	for _, name := range r.Names() {
		data, ok := samples[name]
		if !ok {
			t.Fatalf("no sample for %s", name)
		}
		data["AppURL"], data["Year"] = "https://paytrack.example", 2026
		got, err := r.Render(name, data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for part, s := range map[string]string{"subject": got.Subject, "text": got.Text, "html": got.HTML} {
			if strings.Contains(s, "<no value>") || strings.Contains(s, "%%") || strings.Contains(s, "ZgotmplZ") {
				t.Errorf("%s %s has unfilled placeholders", name, part)
			}
		}
		if got.Subject == "" || !strings.Contains(got.HTML, "#6472f3") {
			t.Errorf("%s: missing subject or design tokens", name)
		}
		if out != "" {
			_ = os.WriteFile(filepath.Join(out, name+".html"), []byte(got.HTML), 0o644)
		}
	}
}

func TestHTMLEscapesData(t *testing.T) {
	r, _ := New()
	d := Samples("https://x")["verify_email"]
	d["BusinessName"], d["AppURL"], d["Year"] = `<script>alert(1)</script>`, "https://x", 2026
	got, err := r.Render("verify_email", d)
	if err != nil || strings.Contains(got.HTML, "<script>alert") {
		t.Fatalf("data not escaped: %v", err)
	}
}
