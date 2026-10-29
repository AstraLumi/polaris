package main

// A font's line spacing as Word on Windows draws it (docs/EDITOR.md, "Line
// height"): the editor's "single" spacing is not the browser's `normal`
// (hhea) but GDI's: OS/2 usWinAscent + usWinDescent plus the external
// leading, max(0, hhea.lineGap - ((winAscent + winDescent) - (hheaAscender -
// hheaDescender))), over unitsPerEm.
//
//	GET /api/fonts/line-metrics?family=X   {family, win, hhea}, 404 when unknown
//
// A family in the fonts library is read from its stored file; any other is a
// Google font: its CSS (asked with Go's own User-Agent, which gets TTF
// files) names a file, which is read once and kept in memory and in
// <data dir>/font-line-metrics.json (so it works offline afterwards).

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	googleFontsCSS     = "https://fonts.googleapis.com/css"
	maxMetricsFontSize = 16 << 20
	metricsCacheFile   = "font-line-metrics.json"
)

type lineMetrics struct {
	Win  float64 `json:"win"`
	Hhea float64 `json:"hhea"`
}

var errNoMetrics = errors.New("no line metrics in this font")

// parseLineMetrics reads head, hhea and OS/2 from a TTF, OTF or WOFF.
func parseLineMetrics(data []byte) (lineMetrics, error) {
	format, err := sniffFont(data)
	if err != nil {
		return lineMetrics{}, err
	}
	var tables map[string][]byte
	switch format {
	case "ttf", "otf":
		tables, err = sfntTables(data)
	case "woff":
		tables, err = woffTables(data)
	default:
		return lineMetrics{}, errNoMetrics
	}
	if err != nil {
		return lineMetrics{}, err
	}
	head, hhea, os2 := tables["head"], tables["hhea"], tables["OS/2"]
	if len(head) < 20 || len(hhea) < 10 {
		return lineMetrics{}, errNoMetrics
	}
	upm := float64(binary.BigEndian.Uint16(head[18:]))
	if upm < 16 {
		return lineMetrics{}, errNoMetrics
	}
	asc := float64(int16(binary.BigEndian.Uint16(hhea[4:])))
	desc := float64(int16(binary.BigEndian.Uint16(hhea[6:])))
	gap := float64(int16(binary.BigEndian.Uint16(hhea[8:])))
	out := lineMetrics{Hhea: (asc - desc + gap) / upm}
	out.Win = out.Hhea
	if len(os2) >= 78 {
		wa := float64(binary.BigEndian.Uint16(os2[74:]))
		wd := float64(binary.BigEndian.Uint16(os2[76:]))
		if wa+wd > 0 {
			lead := gap - ((wa + wd) - (asc - desc))
			if lead < 0 {
				lead = 0
			}
			out.Win = (wa + wd + lead) / upm
		}
	}
	round := func(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }
	out.Win, out.Hhea = round(out.Win), round(out.Hhea)
	return out, nil
}

type fontMetricsCache struct {
	mu   sync.Mutex
	mem  map[string]lineMetrics
	read bool
}

// metricsFor gives a family's metrics: the library's file, else Google's.
func (l *fontLibrary) metricsFor(family string) (lineMetrics, bool) {
	key := strings.ToLower(strings.TrimSpace(family))
	if key == "" || len(key) > 100 {
		return lineMetrics{}, false
	}
	if m, ok := l.fileMetrics(key); ok {
		return m, true
	}
	if b, ok := bundledFonts[key]; ok && b.Metrics != nil {
		return *b.Metrics, true // bundled fonts need no Google
	}
	c := &l.metrics
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.read {
		c.read = true
		c.mem = map[string]lineMetrics{}
		if p := l.metricsPath(); p != "" {
			if b, err := os.ReadFile(p); err == nil {
				_ = json.Unmarshal(b, &c.mem)
			}
		}
	}
	if m, ok := c.mem[key]; ok {
		return m, true
	}
	m, err := l.googleMetrics(strings.TrimSpace(family))
	if err != nil {
		return lineMetrics{}, false
	}
	c.mem[key] = m
	if p := l.metricsPath(); p != "" {
		if b, err := json.Marshal(c.mem); err == nil {
			_ = os.WriteFile(p, b, 0o644)
		}
	}
	return m, true
}

func (l *fontLibrary) metricsPath() string {
	if l.mgr == nil || l.mgr.dataDir == "" {
		return ""
	}
	return filepath.Join(l.mgr.dataDir, metricsCacheFile)
}

// fileMetrics: the library's regular face of the family (else any face).
func (l *fontLibrary) fileMetrics(key string) (lineMetrics, bool) {
	rows, err := l.db.Query(`SELECT f.file FROM font_files f JOIN font_families fam ON fam.id = f.family_id
		WHERE lower(fam.name) = ? ORDER BY f.italic, abs(f.weight - 400), f.id`, key)
	if err != nil {
		return lineMetrics{}, false
	}
	var files []string
	for rows.Next() {
		var f string
		if rows.Scan(&f) == nil {
			files = append(files, f)
		}
	}
	rows.Close()
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(l.dir, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		if m, err := parseLineMetrics(data); err == nil {
			return m, true
		}
	}
	return lineMetrics{}, false
}

var googleFileRE = regexp.MustCompile(`url\((https?://[^)\s'"]+)\)`)

// errNotGoogle: Google's CSS has no such family (as opposed to being
// unreachable).
var errNotGoogle = errors.New("not a Google font")

// googleTTF fetches a Google family's regular TTF.
func (l *fontLibrary) googleTTF(family string) ([]byte, error) {
	base := l.googleBase
	if base == "" {
		base = googleFontsCSS
	}
	client := &http.Client{Timeout: 10 * time.Second}
	get := func(u string, limit int64) ([]byte, error) {
		resp, err := client.Get(u)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
			return nil, errNotGoogle
		}
		if resp.StatusCode != http.StatusOK {
			return nil, errNoMetrics
		}
		return io.ReadAll(io.LimitReader(resp.Body, limit))
	}
	css, err := get(base+"?family="+url.QueryEscape(family), 1<<20)
	if err != nil {
		return nil, err
	}
	m := googleFileRE.FindSubmatch(css)
	if m == nil {
		return nil, errNoMetrics
	}
	return get(string(m[1]), maxMetricsFontSize)
}

func (l *fontLibrary) googleMetrics(family string) (lineMetrics, error) {
	data, err := l.googleTTF(family)
	if err != nil {
		return lineMetrics{}, err
	}
	return parseLineMetrics(data)
}

func (l *fontLibrary) lineMetricsHandler(w http.ResponseWriter, r *http.Request) {
	family := r.URL.Query().Get("family")
	m, ok := l.metricsFor(family)
	if !ok {
		http.Error(w, "no line metrics for this font", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"family": family, "win": m.Win, "hhea": m.Hhea})
}
