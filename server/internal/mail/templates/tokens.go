package templates

import "strings"

// Design tokens copied from the dashboard (client/src/app/globals.css and the
// receipt page) so emails share the product's visual language. Templates
// reference them as %%name%%; they are substituted before parsing.
var tokens = map[string]string{
	"primary": "#6472f3", // --primary (paytrack indigo)
	"fg":      "#0f1222", // --foreground
	"body":    "#3d4461", // paragraph text
	"muted":   "#657090", // --muted-foreground
	"border":  "#e9ebf4", // card border
	"surface": "#f9faff", // table header / muted surface
	"bg":      "#f6f7ff",
	"success": "#16a34a",
	"danger":  "#b42318",
	"font":    "'Figtree', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif",
}

// Reusable inline style strings (email clients ignore most <style> rules).
var styles = map[string]string{
	"P":     "margin:0 0 16px 0;font-size:15px;line-height:1.6;color:%%body%%;",
	"SMALL": "margin:0 0 12px 0;font-size:13px;line-height:1.6;color:%%muted%%;",
	"H1":    "margin:0 0 12px 0;font-size:22px;line-height:1.3;font-weight:700;letter-spacing:-0.01em;color:%%fg%%;",
	"BOX":   "border:1px solid %%border%%;border-radius:12px;padding:14px 16px;",
	"LABEL": "font-size:12px;color:%%muted%%;",
	"MONO":  "font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;",
}

func expand(src string) string {
	pairs := make([]string, 0, 2*(len(tokens)+len(styles)))
	for k, v := range styles {
		pairs = append(pairs, "%%"+k+"%%", v)
	}
	s := strings.NewReplacer(pairs...).Replace(src)
	pairs = pairs[:0]
	for k, v := range tokens {
		pairs = append(pairs, "%%"+k+"%%", v)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
