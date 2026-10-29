package main

// "Fonts and licenses" for one book (docs/EDITOR.md): GET
// /api/books/{volume}/fonts gives each family the book names (fontlib.go's
// bookFontUses) with where the font comes from, its license and a verdict.
//
// Source, in this order: the fonts library, the app's bundled fonts, the
// system fonts most computers have (proprietary, never asked of Google), a
// Google family (its copyright, license and license URL are read from its
// TTF's name table, kept in <data dir>/font-licenses.json), else unknown.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const licenseCacheFile = "font-licenses.json"

// Sources and verdicts.
const (
	srcLibrary = "library"
	srcBundled = "bundled"
	srcGoogle  = "google"
	srcSystem  = "system"
	srcUnknown = "unknown"

	verdictOK    = "ok"
	verdictCheck = "check"
	verdictWarn  = "warn"
)

// systemFonts mirrors SYSTEM_FONTS in frontend/src/editor/fonts.js: fonts
// most computers have, which are proprietary. Change both together.
var systemFonts = map[string]bool{}

func init() {
	for _, f := range []string{
		"Times New Roman", "Times", "Arial", "Helvetica", "Calibri", "Cambria", "Georgia", "Garamond", "Book Antiqua",
		"Palatino", "Palatino Linotype", "Verdana", "Tahoma", "Trebuchet MS", "Courier New", "Courier", "Segoe UI",
		"Aptos", "Constantia", "Candara", "Consolas", "Century", "Century Schoolbook", "Baskerville", "Didot",
		"Hoefler Text", "Optima", "Gill Sans", "Symbol", "Wingdings", "Comic Sans MS", "Impact",
	} {
		systemFonts[strings.ToLower(f)] = true
	}
}

// bookFontOut is a family a book names, with its source and license.
type bookFontOut struct {
	bookFontUse
	Source     string       `json:"source"`      // library, bundled, google, system, unknown
	SourceURL  string       `json:"source_url"`  // the library record's "Source"
	LicenseID  string       `json:"license_id"`  // OFL-1.1, Apache-2.0, UFL-1.0 or ""
	License    string       `json:"license"`     // the license as the font or record names it
	LicenseURL string       `json:"license_url"` // http(s) only
	Copyright  string       `json:"copyright"`
	Designer   string       `json:"designer"`
	Allowed    *fontUsesOut `json:"allowed,omitempty"` // a library font's uses
	Verdict    string       `json:"verdict"`           // ok, check, warn
	Reason     string       `json:"reason"`            // why: open, uses, unknown, embedding, proprietary
}

// fontLicenseInfo is what a Google font file says about itself.
type fontLicenseInfo struct {
	Copyright  string `json:"copyright"`
	License    string `json:"license"`
	LicenseURL string `json:"license_url"`
	Designer   string `json:"designer"`
}

type fontLicenseCache struct {
	mu   sync.Mutex
	mem  map[string]fontLicenseInfo
	read bool
}

func (l *fontLibrary) licensePath() string {
	if l.mgr == nil || l.mgr.dataDir == "" {
		return ""
	}
	return filepath.Join(l.mgr.dataDir, licenseCacheFile)
}

// googleLicense: a Google family's license strings, from the disk cache or
// its TTF. known is false when Google doesn't have it; err when it couldn't
// be asked (offline).
func (l *fontLibrary) googleLicense(family string) (info fontLicenseInfo, known bool, err error) {
	key := strings.ToLower(strings.TrimSpace(family))
	c := &l.licenses
	c.mu.Lock()
	if !c.read {
		c.read = true
		c.mem = map[string]fontLicenseInfo{}
		if p := l.licensePath(); p != "" {
			if b, e := os.ReadFile(p); e == nil {
				_ = json.Unmarshal(b, &c.mem)
			}
		}
	}
	if v, ok := c.mem[key]; ok {
		c.mu.Unlock()
		return v, true, nil
	}
	c.mu.Unlock()

	data, e := l.googleTTF(strings.TrimSpace(family))
	if e == errNotGoogle {
		return info, false, nil
	}
	if e != nil {
		return info, true, e
	}
	fi, e := parseFont(data)
	if e != nil || !fi.Read {
		return info, true, errNoMetrics
	}
	info = fontLicenseInfo{Copyright: fi.Copyright, License: fi.License, LicenseURL: fi.LicenseURL, Designer: fi.Designer}
	c.mu.Lock()
	c.mem[key] = info
	if p := l.licensePath(); p != "" {
		if b, e := json.Marshal(c.mem); e == nil {
			_ = os.WriteFile(p, b, 0o644)
		}
	}
	c.mu.Unlock()
	return info, true, nil
}

// licenseIDOf recognises the open licenses in a font's license text or URL.
func licenseIDOf(text, url string) string {
	s := strings.ToLower(text + " " + url)
	switch {
	case strings.Contains(s, "open font license") || strings.Contains(s, "scripts.sil.org/ofl") || strings.Contains(s, "openfontlicense") || strings.Contains(s, "ofl-1.1"):
		return "OFL-1.1"
	case strings.Contains(s, "apache"):
		return "Apache-2.0"
	case strings.Contains(s, "ubuntu font licen") || strings.Contains(s, "ubuntu.com/legal/font-licence"):
		return "UFL-1.0"
	}
	return ""
}

// safeURL keeps only http(s) addresses (the text comes from font files).
func safeURL(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
		return s
	}
	return ""
}

// describeFonts gives each use its source, license and verdict.
func (l *fontLibrary) describeFonts(uses []bookFontUse) []bookFontOut {
	out := make([]bookFontOut, len(uses))
	var lib map[string]fontFamilyOut
	if l != nil {
		if list, err := l.list(); err == nil {
			lib = map[string]fontFamilyOut{}
			for _, f := range list {
				lib[strings.ToLower(f.Name)] = f
			}
		}
	}
	var wg sync.WaitGroup
	for i, u := range uses {
		o := bookFontOut{bookFontUse: u}
		key := strings.ToLower(strings.TrimSpace(u.Family))
		if f, ok := lib[key]; ok {
			describeLibrary(&o, f)
		} else if b, ok := bundledFonts[key]; ok {
			o.Source, o.LicenseID, o.Copyright, o.Designer = srcBundled, b.License, b.Copyright, b.Designer
			o.License, o.LicenseURL = licenseName(b.License), safeURL(b.LicenseURL)
			o.Verdict, o.Reason = verdictOK, "open"
		} else if systemFonts[key] {
			o.Source, o.Verdict, o.Reason = srcSystem, verdictWarn, "proprietary"
		} else if l == nil || key == "" {
			o.Source, o.Verdict, o.Reason = srcUnknown, verdictCheck, "unknown"
		} else {
			wg.Add(1)
			go func(i int, o bookFontOut) {
				defer wg.Done()
				info, known, err := l.googleLicense(o.Family)
				switch {
				case !known:
					o.Source, o.Verdict, o.Reason = srcUnknown, verdictCheck, "unknown"
				case err != nil: // Google unreachable: what is known
					o.Source, o.Verdict, o.Reason = srcGoogle, verdictCheck, "unknown"
				default:
					o.Source = srcGoogle
					o.Copyright, o.License, o.Designer = info.Copyright, info.License, info.Designer
					o.LicenseURL = safeURL(info.LicenseURL)
					o.LicenseID = licenseIDOf(info.License, info.LicenseURL)
					if o.LicenseID != "" {
						o.Verdict, o.Reason = verdictOK, "open"
					} else {
						o.Verdict, o.Reason = verdictCheck, "unknown"
					}
				}
				out[i] = o
			}(i, o)
			continue
		}
		out[i] = o
	}
	wg.Wait()
	return out
}

func licenseName(id string) string {
	switch id {
	case "OFL-1.1":
		return "SIL Open Font License 1.1"
	case "Apache-2.0":
		return "Apache License 2.0"
	case "UFL-1.0":
		return "Ubuntu Font License 1.0"
	}
	return id
}

func describeLibrary(o *bookFontOut, f fontFamilyOut) {
	o.Source = srcLibrary
	o.SourceURL = safeURL(f.Source)
	if o.SourceURL == "" {
		o.SourceURL = strings.TrimSpace(f.Source)
	}
	allowed := f.Uses
	o.Allowed = &allowed
	forbids := false
	for _, file := range f.Files {
		if o.Copyright == "" {
			o.Copyright = file.Copyright
		}
		if o.Designer == "" {
			o.Designer = file.Designer
		}
		if o.License == "" {
			o.License = file.License
		}
		if o.LicenseURL == "" {
			o.LicenseURL = safeURL(file.LicenseURL)
		}
		if file.forbidsEmbedding() {
			forbids = true
		}
	}
	switch f.LicenseKind {
	case "ofl":
		o.LicenseID = "OFL-1.1"
	case "apache":
		o.LicenseID = "Apache-2.0"
	default:
		o.LicenseID = licenseIDOf(o.License, o.LicenseURL)
	}
	if o.LicenseID != "" {
		o.License = licenseName(o.LicenseID)
	}
	open := o.LicenseID != ""
	switch {
	case forbids:
		o.Verdict, o.Reason = verdictCheck, "embedding"
	case open:
		o.Verdict, o.Reason = verdictOK, "open"
	case f.LicenseKind == "" && !f.Uses.Print && !f.Uses.Ebook:
		o.Verdict, o.Reason = verdictCheck, "unknown"
	case f.Uses.Print && f.Uses.Ebook:
		o.Verdict, o.Reason = verdictOK, "uses"
	default:
		o.Verdict, o.Reason = verdictCheck, "uses"
	}
}
