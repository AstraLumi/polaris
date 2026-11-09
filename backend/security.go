package main

// What stands between the browser and the API. Polaris has no login: who can
// reach it is decided by the address it listens on and whatever is put in
// front of it. That leaves the browser itself, where any other site that is
// open can send requests to Polaris's address. This file refuses those.

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// Form uploads. ParseMultipartForm's own number is only how much it keeps in
// memory (the rest goes to temporary files), so the body is capped first.
const (
	maxPictureForm = 32 << 20  // a form with one picture
	maxGalleryForm = 256 << 20 // the wiki gallery takes several at once
)

// parsePictureForm reads a form that carries one picture. w may be nil
// (then the connection just isn't closed early on a body that is too big).
func parsePictureForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxPictureForm)
	return r.ParseMultipartForm(maxFormMemory)
}

// guard wraps the whole server:
//
//   - a request that changes something is refused when the browser says it
//     comes from another site. Without this, a page open in another tab
//     could post a form to Polaris's address and add, change or import
//     things. Browsers say so with Sec-Fetch-Site (on https and localhost)
//     or with Origin, which must then name this server. A request with
//     neither is not from a browser page (curl, a script) and passes;
//   - with ALLOWED_HOSTS set, every request must be addressed to one of
//     those names. That closes "DNS rebinding", where another site's name
//     is pointed at this machine so its pages may read the answers too;
//   - nothing is ever guessed to be a page or a script from its contents.
func guard(next http.Handler, allowedHosts map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(allowedHosts) > 0 && !allowedHosts[hostName(r.Host)] {
			http.Error(w, "this address is not one Polaris answers to (see ALLOWED_HOSTS)", http.StatusForbidden)
			return
		}
		if changesThings(r.Method) && fromAnotherSite(r) {
			http.Error(w, "requests from other sites are refused", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func changesThings(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func fromAnotherSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none": // the app itself; typed or bookmarked
		return false
	case "cross-site", "same-site":
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return true // "null" and the like
	}
	// A proxy in front may put its own name in Host and pass the one the
	// browser used along.
	for _, h := range []string{r.Host, r.Header.Get("X-Forwarded-Host")} {
		if h != "" && strings.EqualFold(u.Host, strings.TrimSpace(strings.Split(h, ",")[0])) {
			return false
		}
	}
	return true
}

// hostName is a Host header without its port, lower case.
func hostName(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

// allowedHostsFromEnv reads ALLOWED_HOSTS: names separated by commas, with
// or without a port. Empty means any name is answered.
func allowedHostsFromEnv() map[string]bool {
	out := map[string]bool{}
	for _, h := range strings.Split(os.Getenv("ALLOWED_HOSTS"), ",") {
		if h = strings.TrimSpace(h); h != "" {
			out[hostName(h)] = true
		}
	}
	return out
}

var inlineScriptRE = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// pagePolicy is the page's Content-Security-Policy: scripts only from
// Polaris itself, so text that somehow got onto a page as markup still
// can't run anything or send anything elsewhere. The page's own inline
// script (the theme, before first paint) is let through by its hash.
// Styles stay open to inline ones (Vue's :style, the editor's page layout)
// and Google Fonts, which the interface and the book fonts load.
func pagePolicy(staticFS fs.FS) string {
	scripts := "'self' 'wasm-unsafe-eval'"
	if page, err := fs.ReadFile(staticFS, "index.html"); err == nil {
		for _, m := range inlineScriptRE.FindAllSubmatch(page, -1) {
			sum := sha256.Sum256(m[1])
			scripts += " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
		}
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + scripts,
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' data: blob: https://fonts.gstatic.com",
		"img-src 'self' data: blob:",
		"media-src 'self' data: blob:",
		"connect-src 'self' https://fonts.googleapis.com https://fonts.gstatic.com",
		"worker-src 'self' blob:",
		"frame-src 'self' blob:",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'self'",
	}, "; ")
}
