package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Merriweather-like numbers: hhea 1.257, win 1.338.
var metricsTestFont = testFont{
	family: "Metric Serif", style: "Regular", weight: 400,
	upm: 1000, hheaAsc: 984, hheaDesc: -273, hheaGap: 0, winA: 1100, winD: 238,
}

func TestParseLineMetrics(t *testing.T) {
	m, err := parseLineMetrics(buildSfnt(metricsTestFont))
	if err != nil {
		t.Fatal(err)
	}
	if m.Hhea != 1.257 || m.Win != 1.338 {
		t.Errorf("got %+v, want hhea 1.257 win 1.338", m)
	}
	// External leading: the gap beyond what win already covers is added.
	f := metricsTestFont
	f.hheaGap, f.winA, f.winD = 300, 900, 200 // win 1100 < hhea content 1257: the gap plus the shortfall
	m, _ = parseLineMetrics(buildSfnt(f))
	if m.Win != 1.557 || m.Hhea != 1.557 {
		t.Errorf("shortfall case: %+v", m)
	}
	f.hheaAsc, f.hheaDesc, f.hheaGap = 800, -200, 300 // content 1000, win 1100: leading 300 - 100
	m, _ = parseLineMetrics(buildSfnt(f))
	if m.Win != 1.3 {
		t.Errorf("leading case: %+v, want win 1.3", m)
	}
	if _, err := parseLineMetrics(buildSfnt(testFont{family: "No Hhea", style: "Regular", weight: 400})); err == nil {
		t.Error("a font without hhea has no metrics")
	}
	if _, err := parseLineMetrics([]byte("not a font")); err == nil {
		t.Error("garbage has no metrics")
	}
}

func getMetrics(t *testing.T, h http.Handler, family string) (int, map[string]any) {
	t.Helper()
	rec := do(t, h, "GET", "/api/fonts/line-metrics?family="+family, nil, "")
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestLineMetricsEndpoint(t *testing.T) {
	mgr, h := newTestServer(t)
	// A library family.
	if code, body := uploadFonts(t, h, nil, map[string][]byte{"M.ttf": buildSfnt(metricsTestFont)}); code != 200 {
		t.Fatal(code, body)
	}
	code, out := getMetrics(t, h, "metric+serif")
	if code != 200 || out["win"] != 1.338 || out["hhea"] != 1.257 {
		t.Fatalf("library: %d %v", code, out)
	}
	// A Google family, from a fake Google; asked once, then kept (also on disk).
	asked := 0
	google := buildSfnt(testFont{family: "Gfont", style: "Regular", weight: 400, upm: 2000, hheaAsc: 1900, hheaDesc: -500, winA: 2000, winD: 600})
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css":
			asked++
			if r.URL.Query().Get("family") != "Some Google Font" {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, "@font-face { font-family: 'X'; src: url(%s/f.ttf) format('truetype'); }", srv.URL)
		case "/f.ttf":
			w.Write(google)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	mgr.fonts.googleBase = srv.URL + "/css"
	code, out = getMetrics(t, h, "Some+Google+Font")
	if code != 200 || out["win"] != 1.3 || out["hhea"] != 1.2 {
		t.Fatalf("google: %d %v", code, out)
	}
	getMetrics(t, h, "some+google+font")
	if asked != 1 {
		t.Errorf("Google asked %d times, want 1", asked)
	}
	if b, err := os.ReadFile(filepath.Join(mgr.dataDir, metricsCacheFile)); err != nil || len(b) == 0 {
		t.Errorf("not kept on disk: %v", err)
	}
	// Unknown, empty: 404.
	if code, _ := getMetrics(t, h, "Nobody+Has+This"); code != 404 {
		t.Errorf("unknown family: %d", code)
	}
	if code, _ := getMetrics(t, h, ""); code != 404 {
		t.Errorf("no family: %d", code)
	}
}
