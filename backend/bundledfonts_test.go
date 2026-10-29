package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBundledFontsRegistry(t *testing.T) {
	for _, f := range []string{
		"Merriweather", "Literata", "Crimson Pro", "Cinzel", "Cormorant Garamond", "Playfair Display",
		"Orbitron", "Turret Road", "Exo 2", "Rajdhani", "Share Tech Mono", "VT323", "Press Start 2P",
		"Caveat", "Patrick Hand", "Kalam", "Bangers", "Permanent Marker",
		"Noto Serif JP", "Noto Sans JP", "Zen Old Mincho", "Shippori Mincho",
		"Noto Sans Symbols", "Noto Sans Symbols 2", "Noto Sans Math", "Noto Emoji",
	} {
		b, ok := bundledFonts[strings.ToLower(f)]
		if !ok {
			t.Errorf("%s is not in the registry", f)
			continue
		}
		if b.License != "OFL-1.1" && b.License != "Apache-2.0" {
			t.Errorf("%s: license %q", f, b.License)
		}
		if b.Copyright == "" || b.LicenseURL == "" {
			t.Errorf("%s: missing copyright or license URL", f)
		}
		if b.Metrics == nil || b.Metrics.Win <= 0 {
			t.Errorf("%s: no line metrics", f)
		}
	}
}

func TestBundledFontListedAsBundled(t *testing.T) {
	mgr, h := newTestServer(t)
	_ = mgr
	id, vol, _ := fontStory(t, h, "Bundled Story")
	settings := `{"settings":{"page_type":"a5","styles":[
		{"id":"normal","name":"Normal","kind":"paragraph","builtin":true,"text":{"font":"merriweather"}},
		{"id":"jp","name":"Japanese","kind":"paragraph","text":{"font":"Noto Serif JP"}}]}}`
	if rec := do(t, h, "PUT", fmt.Sprintf("/api/s/%s/books/%d", id, vol), strings.NewReader(settings), "application/json"); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	rec := do(t, h, "GET", fmt.Sprintf("/api/s/%s/books/%d/fonts", id, vol), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out []bookFontOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, o := range out {
		k := strings.ToLower(o.Family)
		if k != "merriweather" && k != "noto serif jp" {
			continue
		}
		seen++
		if o.Source != "bundled" || o.Verdict != "ok" || o.LicenseID != "OFL-1.1" || o.Copyright == "" {
			t.Errorf("%s: %+v", o.Family, o)
		}
	}
	if seen != 2 {
		t.Errorf("saw %d of the two bundled families in %s", seen, rec.Body.String())
	}
}

func TestBundledFontMetricsNeedNoGoogle(t *testing.T) {
	mgr, _ := newTestServer(t)
	mgr.fonts.googleBase = "http://127.0.0.1:1/css" // unreachable: must not be asked
	m, ok := mgr.fonts.metricsFor("Merriweather")
	if !ok || m.Win <= 0 || m.Hhea <= 0 {
		t.Fatalf("metrics: %+v %v", m, ok)
	}
}
