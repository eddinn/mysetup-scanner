// Package page is the app's graphical front end: one page, served to the member's own browser from this computer only, showing
// the report with buttons to copy it, save it and finish. It is an attack surface and is hardened as the design asks (MySetup's
// docs/scanner/design.md, Front ends): 127.0.0.1 only, a random port, a one-time token in every address, the Host header checked
// against DNS rebinding, requests that change anything accepted only from the page itself, a content policy that lets the page
// load nothing from anywhere else, and an end when the member is done or after a time without use.
package page

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Content is what the page shows.
type Content struct {
	Version string
	Report  string
	Notes   []string
	// BuildsURL is where the member finds their builds on MySetup, linked from the steps.
	BuildsURL string
}

// Server is one page for one report.
type Server struct {
	content Content
	token   string
	host    string
	life    time.Duration
	gone    time.Duration
	start   time.Time

	mu       sync.Mutex
	loaded   bool
	seen     time.Time
	done     chan struct{}
	doneOnce sync.Once
}

// New prepares a page on a listener the caller opened (net.Listen on 127.0.0.1:0), with its own random token. It ends life after
// it starts, and gone after its tab stopped checking in (the page checks in every 20 seconds while it is open).
func New(content Content, listener net.Listener, life, gone time.Duration) (*Server, error) {
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !addr.IP.IsLoopback() {
		return nil, errors.New("page: the listener must be on the loopback address")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}

	now := time.Now()

	return &Server{content: content, token: hex.EncodeToString(secret), host: addr.String(), life: life, gone: gone, start: now, seen: now,
		done: make(chan struct{})}, nil
}

// URL is the page's address, token included.
func (s *Server) URL() string { return "http://" + s.host + "/t/" + s.token + "/" }

// Serve answers requests until the member is done, the tab has gone, the page's time is up, or ctx ends.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	server := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second,
		MaxHeaderBytes: 16 * 1024}
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(listener) }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-errs:
			return err
		case <-ctx.Done():
		case <-s.done:
		case <-ticker.C:
			s.mu.Lock()
			tabGone := s.loaded && time.Since(s.seen) >= s.gone
			s.mu.Unlock()
			if !tabGone && time.Since(s.start) < s.life {
				continue
			}
		}
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = server.Shutdown(shutdown)
		cancel()

		return nil
	}
}

// Done is closed when the member has finished.
func (s *Server) Done() <-chan struct{} { return s.done }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nonce := randomNonce()
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; style-src 'nonce-"+nonce+"'; connect-src 'self'; "+
		"base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")

	// Another name for this address is another site, as in DNS rebinding: only 127.0.0.1 and this port.
	if r.Host != s.host {
		http.Error(w, "Misdirected request.", http.StatusMisdirectedRequest)

		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/t/")
	token, action, _ := strings.Cut(rest, "/")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		http.NotFound(w, r)

		return
	}
	s.mu.Lock()
	s.seen = time.Now()
	if action == "" {
		s.loaded = true
	}
	s.mu.Unlock()

	switch {
	case action == "alive" && r.Method == http.MethodGet:
		w.WriteHeader(http.StatusNoContent)
	case action == "" && r.Method == http.MethodGet:
		h.Set("Content-Type", "text/html; charset=utf-8")
		_ = pageTemplate.Execute(w, struct {
			Content
			Nonce string
		}{s.content, nonce})
	case action == "report.txt" && r.Method == http.MethodGet:
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Disposition", `attachment; filename="mysetup-report.txt"`)
		fmt.Fprint(w, s.content.Report)
	case action == "done" && r.Method == http.MethodPost:
		// Only the page itself may end it: a request from another site carries another Origin.
		if r.Header.Get("Origin") != "http://"+s.host || (r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin") {
			http.Error(w, "Forbidden.", http.StatusForbidden)

			return
		}
		w.WriteHeader(http.StatusNoContent)
		s.doneOnce.Do(func() { close(s.done) })
	default:
		http.NotFound(w, r)
	}
}

// randomNonce is 128 random bits in hex, which html/template writes into an attribute as it is (base64's "+" would become "&#43;").
func randomNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MySetup scan</title>
<style nonce="{{.Nonce}}">
:root { color-scheme: dark; }
body { margin: 0; background: #09090b; color: #e4e4e7; font: 15px/1.6 system-ui, -apple-system, "Segoe UI", sans-serif; }
main { max-width: 52rem; margin: 0 auto; padding: 2rem 1rem 3rem; }
h1 { font-size: 1.6rem; margin: 0 0 .25rem; color: #fff; }
.lead { color: #a1a1aa; margin: 0 0 1.5rem; }
.notes { border: 1px solid #78350f; background: #451a0333; color: #fde68a; border-radius: .75rem; padding: .75rem 1rem; margin: 0 0 1.25rem; }
.notes p { margin: .25rem 0; }
.actions { display: flex; flex-wrap: wrap; gap: .75rem; margin: 0 0 1rem; }
button, a.button { font: inherit; font-weight: 600; border-radius: .75rem; padding: .6rem 1.1rem; cursor: pointer; text-decoration: none; }
.primary { background: #6366f1; color: #fff; border: 0; }
.primary:hover { background: #818cf8; }
.secondary { background: #18181b; color: #e4e4e7; border: 1px solid #3f3f46; }
.secondary:hover { border-color: #52525b; }
pre { background: #18181b; border: 1px solid #27272a; border-radius: .75rem; padding: 1rem; overflow-x: auto; font: 13px/1.5 ui-monospace, "Cascadia Mono", Consolas, monospace; color: #d4d4d8; }
.how { color: #a1a1aa; font-size: .95rem; }
.status { display: block; min-height: 1.5rem; color: #6ee7b7; margin: .4rem 0 0; }
h2 { font-size: 1.15rem; color: #fff; margin: 1.5rem 0 .75rem; }
.steps { margin: 0 0 1.25rem; padding-left: 1.6rem; }
.steps li { margin: 0 0 1.1rem; padding-left: .25rem; }
.step { font-weight: 600; color: #fff; }
.hint { display: block; color: #a1a1aa; font-size: .95rem; margin-top: .3rem; }
kbd { font: 13px ui-monospace, Consolas, monospace; border: 1px solid #3f3f46; border-radius: .3rem; padding: .05rem .35rem; background: #18181b; }
.later { border: 1px solid #3730a3; background: #1e1b4b66; color: #c7d2fe; border-radius: .75rem; padding: .75rem 1rem; margin: 0 0 1.25rem; }
details { margin: 0 0 1.25rem; } summary { cursor: pointer; color: #a5b4fc; font-weight: 600; }
.steps .button, .steps button { margin-top: .4rem; display: inline-block; }
</style>
</head>
<body>
<main>
<h1>MySetup scan</h1>
<p class="lead">Your computer's hardware report, read on this computer just now. Nothing has been sent anywhere.</p>
{{if .Notes}}<div class="notes" role="note">{{range .Notes}}<p>{{.}}</p>{{end}}</div>{{end}}
<h2>Add it to your build in 6 steps</h2>
<ol class="steps">
<li><span class="step">Click this button:</span> <button type="button" class="primary" id="copy">Copy report</button>
<span class="status" id="status" role="status" aria-live="polite"></span></li>
<li><span class="step">Open your builds on MySetup:</span> <a class="button secondary" href="{{.BuildsURL}}" target="_blank" rel="noopener noreferrer">Open mysetup.is in a new tab</a>
<span class="hint">Sign in if it asks.</span></li>
<li><span class="step">Click your build.</span> <span class="hint">No build yet? Click "Create setup" first.</span></li>
<li><span class="step">Scroll down to "Hardware" and find "Scan this computer".</span> Click in the box "Paste your report here", then
paste: press <kbd>Ctrl</kbd>+<kbd>V</kbd> (on a Mac <kbd>Cmd</kbd>+<kbd>V</kbd>).</li>
<li><span class="step">Click "Review the report".</span> Check each part, then click "Add these parts". Nothing changes in your build
until you do.</li>
<li><span class="step">Come back to this tab and click:</span> <button type="button" class="secondary" id="done">Done</button></li>
</ol>
<p class="later">Coming in a later version: sending the report straight to your build with a code from your build page, without copying
and pasting. This version cannot send anything.</p>
<details>
<summary>See the report</summary>
<pre id="report">{{.Report}}</pre>
<p><a class="button secondary" href="report.txt" download>Save it as a file</a></p>
</details>
<p class="how">MySetup scan {{.Version}}. This page is served by the app on your own computer only. The app stops when you click
Done, close this tab, or after 15 minutes.</p>
</main>
<script nonce="{{.Nonce}}">
const status = document.getElementById('status');
document.getElementById('copy').addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(document.getElementById('report').textContent);
    status.textContent = 'Copied. Now do step 2.';
  } catch (e) {
    document.querySelector('details').open = true;
    getSelection().selectAllChildren(document.getElementById('report'));
    status.textContent = 'Your browser did not let this button copy. The report below is now selected: press Ctrl+C (on a Mac Cmd+C) to copy it, then do step 2.';
  }
});
// While this tab is open, it tells the app so; once it closes, the app stops by itself.
const alive = setInterval(() => fetch('alive').catch(() => {}), 20000);
document.getElementById('done').addEventListener('click', async () => {
  clearInterval(alive);
  try { await fetch('done', {method: 'POST'}); } catch (e) {}
  status.textContent = '';
  document.getElementById('done').insertAdjacentText('afterend', ' Finished. The app has stopped, and you can close this tab.');
  for (const b of document.querySelectorAll('button')) b.disabled = true;
});
</script>
</body>
</html>
`))
