package main

// What stands between the browser and the API. Polaris has no login: who can
// reach it is decided by the address it listens on and whatever is put in
// front of it. That leaves the browser itself, where any other site that is
// open can send requests to Polaris's address. This file refuses those.

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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
//   - every request must be addressed to a name Polaris answers to: one
//     that can only mean a machine of yours (safeName), or one listed in
//     ALLOWED_HOSTS. That closes "DNS rebinding", where another site's
//     name is pointed at this machine: to the browser its pages and
//     Polaris are then one site, so they may read the answers, and the
//     check above no longer sees them as another site either. Only the
//     name gives them away. On unless ALLOWED_HOSTS is "*";
//   - nothing is ever guessed to be a page or a script from its contents.
func guard(next http.Handler, allowedHosts map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if host := hostName(r.Host); !allowedHosts["*"] && !allowedHosts[host] && !safeName(host) {
			refuseName(w, r, host)
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

// safeName: a name no other website can make its own, so one that needs no
// listing. A rebinding attack arrives under a name the attacker owns in the
// public DNS, which is never one of these:
//
//   - an address in numbers (192.168.1.20, ::1): not a name at all;
//   - localhost, and names ending in .localhost;
//   - a name without a dot (nas, the name another container calls this
//     one by): nobody can register one;
//   - names ending in .local, .home.arpa or .internal, which are set aside
//     for private networks and are never handed out.
//
// Every other name (a domain of yours, say) has to be in ALLOWED_HOSTS.
func safeName(host string) bool {
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return true
	}
	for _, end := range []string{".localhost", ".local", ".home.arpa", ".internal"} {
		if strings.HasSuffix(host, end) {
			return true
		}
	}
	return false
}

// refuseName answers a request under a name Polaris doesn't answer to, and
// says what to do about it: this is what someone sees after an update when
// they reach Polaris by a domain they never had to list before.
func refuseName(w http.ResponseWriter, r *http.Request, host string) {
	noteRefusedName(host)
	const line = "ALLOWED_HOSTS="
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Error(w, "Polaris does not answer to the name "+strconv.Quote(host)+": add it to "+line+" in its .env and restart (see the README)", http.StatusForbidden)
		return
	}
	name := html.EscapeString(host)
	if name == "" {
		name = "(none)"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, refusedNamePage, name, line)
}

const refusedNamePage = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Polaris</title>
<style>body{font:16px/1.55 system-ui,sans-serif;max-width:40rem;margin:12vh auto;padding:0 1.25rem;color:#222;background:#f6f3ec}
code,pre{font:14px ui-monospace,Consolas,monospace;background:#e9e3d6;border-radius:6px}code{padding:.1em .35em}pre{padding:.8em 1em;overflow:auto}
@media (prefers-color-scheme:dark){body{color:#e8e4da;background:#14151b}code,pre{background:#262832}}</style>
<h1>Polaris doesn't answer to this name</h1>
<p>You reached Polaris as <code>%[1]s</code>, and that name isn't on its list. Your stories are untouched.</p>
<p>If this is your Polaris, add the name to the file <code>.env</code> beside its <code>docker-compose.yml</code> (several names are separated by commas):</p>
<pre>%[2]s%[1]s</pre>
<p>Then restart it: <code>docker compose up -d</code>.</p>
<p>Why: Polaris has no login, so it only answers to names that are surely yours. Otherwise another website could point a name of its own at this machine and read your stories through your browser. Addresses in numbers, <code>localhost</code> and names without a dot always work. The README says more under <code>ALLOWED_HOSTS</code>.</p>
`

// Each refused name is logged once, and only so many of them: the log is
// where someone locked out looks first, and it must not fill with whatever
// names a scanner tries.
var refusedNames = struct {
	sync.Mutex
	seen map[string]bool
}{seen: map[string]bool{}}

func noteRefusedName(host string) {
	refusedNames.Lock()
	defer refusedNames.Unlock()
	if refusedNames.seen[host] || len(refusedNames.seen) >= 20 {
		return
	}
	refusedNames.seen[host] = true
	log.Printf("refused a request addressed to %q: to answer to that name, add it to ALLOWED_HOSTS", host)
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

// allowedHostsFromEnv reads ALLOWED_HOSTS: the names Polaris answers to
// besides the safe ones (safeName), separated by commas, with or without a
// port. "*" answers to any name, as every version before 3.2 did.
func allowedHostsFromEnv() map[string]bool {
	out := map[string]bool{}
	for _, h := range strings.Split(os.Getenv("ALLOWED_HOSTS"), ",") {
		if h = strings.TrimSpace(h); h == "*" {
			out["*"] = true
		} else if h != "" {
			out[hostName(h)] = true
		}
	}
	return out
}

// describeAllowedHosts is a line for the log at start, so the names are
// there to read when someone can't get in.
func describeAllowedHosts(allowed map[string]bool) string {
	if allowed["*"] {
		return "answering to any name (ALLOWED_HOSTS=*): another website can reach the stories through a name it points here"
	}
	names := make([]string, 0, len(allowed))
	for n := range allowed {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "answering to localhost, addresses in numbers and names without a dot; a domain has to be listed in ALLOWED_HOSTS"
	}
	return "answering to " + strings.Join(names, ", ") + ", and to localhost, addresses in numbers and names without a dot"
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
