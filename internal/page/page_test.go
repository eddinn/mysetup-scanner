package page

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

const madeUpReport = "mysetup-report 1\nsource=app/0.1.0\ncpu.model=Made-up <script>alert(1)</script> CPU\nmysetup-report-end\n"

func newServer(t *testing.T, life, gone time.Duration) (*Server, net.Listener) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	server, err := New(Content{Version: "0.1.0", Report: madeUpReport, Notes: []string{"A made-up <b>note</b>."}}, listener, life, gone)
	if err != nil {
		t.Fatal(err)
	}

	return server, listener
}

func request(s *Server, method, path string, header map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.Host = s.host
	for name, value := range header {
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)

	return w
}

func base(s *Server) string { return "/t/" + s.token + "/" }

func TestNewRefusesAnAddressOtherThanLoopback(t *testing.T) {
	if _, err := New(Content{}, fakeListener{&net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 8080}}, time.Minute, time.Minute); err == nil {
		t.Fatal("a listener on another address was accepted")
	}
}

func TestEachPageHasItsOwnToken(t *testing.T) {
	first, _ := newServer(t, time.Minute, time.Minute)
	second, _ := newServer(t, time.Minute, time.Minute)
	if len(first.token) != 64 || first.token == second.token {
		t.Fatalf("tokens %q and %q", first.token, second.token)
	}
	if !strings.HasPrefix(first.URL(), "http://127.0.0.1:") || !strings.HasSuffix(first.URL(), "/t/"+first.token+"/") {
		t.Fatalf("address %q", first.URL())
	}
}

func TestOnlyTheTokenOpensThePage(t *testing.T) {
	s, _ := newServer(t, time.Minute, time.Minute)
	for _, path := range []string{"/", "/t/", "/t/wrong/", "/t/" + s.token[:63] + "/", "/t/" + s.token + "x/", "/favicon.ico"} {
		if got := request(s, http.MethodGet, path, nil).Code; got != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, got)
		}
	}
	if s.loaded {
		t.Error("a request without the token counted as the page loading")
	}
}

func TestAnotherHostNameIsRefused(t *testing.T) {
	s, _ := newServer(t, time.Minute, time.Minute)
	_, port, _ := net.SplitHostPort(s.host)
	for _, host := range []string{"localhost:" + port, "attacker.example:" + port, "127.0.0.1", "127.0.0.1:1"} {
		r := httptest.NewRequest(http.MethodGet, base(s), nil)
		r.Host = host
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusMisdirectedRequest {
			t.Errorf("Host %s: %d, want 421", host, w.Code)
		}
	}
}

func TestThePageIsLockedDownAndEscapesTheReport(t *testing.T) {
	s, _ := newServer(t, time.Minute, time.Minute)
	w := request(s, http.MethodGet, base(s), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("page: %d", w.Code)
	}
	header := w.Header()
	policy := header.Get("Content-Security-Policy")
	match := regexp.MustCompile(`script-src 'nonce-([A-Za-z0-9+/]{22})'`).FindStringSubmatch(policy)
	if match == nil || !strings.HasPrefix(policy, "default-src 'none';") || !strings.Contains(policy, "frame-ancestors 'none'") {
		t.Fatalf("policy %q", policy)
	}
	body := w.Body.String()
	if strings.Count(body, `nonce="`+match[1]+`"`) != 2 || strings.Count(body, "nonce=") != 2 {
		t.Error("the style and script do not carry this response's nonce")
	}
	for name, want := range map[string]string{"X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer",
		"Cache-Control": "no-store", "X-Frame-Options": "DENY", "Content-Type": "text/html; charset=utf-8"} {
		if got := header.Get(name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
	if strings.Contains(body, "<script>alert(1)") || !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("the report is not escaped")
	}
	if strings.Contains(body, "<b>note</b>") || !strings.Contains(body, "&lt;b&gt;note&lt;/b&gt;") {
		t.Error("the notes are not escaped")
	}
	if again := request(s, http.MethodGet, base(s), nil).Header().Get("Content-Security-Policy"); again == policy {
		t.Error("two responses had the same nonce")
	}
}

func TestTheReportDownloadsAsAFile(t *testing.T) {
	s, _ := newServer(t, time.Minute, time.Minute)
	w := request(s, http.MethodGet, base(s)+"report.txt", nil)
	if w.Code != http.StatusOK || w.Body.String() != madeUpReport {
		t.Fatalf("report.txt: %d %q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="mysetup-report.txt"` {
		t.Errorf("Content-Disposition %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type %q", got)
	}
}

func TestOnlyThePageItselfCanFinish(t *testing.T) {
	s, _ := newServer(t, time.Minute, time.Minute)
	origin := "http://" + s.host
	refused := []struct {
		method string
		header map[string]string
	}{
		{http.MethodGet, map[string]string{"Origin": origin}},
		{http.MethodPost, nil},
		{http.MethodPost, map[string]string{"Origin": "null"}},
		{http.MethodPost, map[string]string{"Origin": "https://attacker.example"}},
		{http.MethodPost, map[string]string{"Origin": "http://localhost" + strings.TrimPrefix(s.host, "127.0.0.1")}},
		{http.MethodPost, map[string]string{"Origin": origin, "Sec-Fetch-Site": "cross-site"}},
		{http.MethodPost, map[string]string{"Origin": origin, "Sec-Fetch-Site": "same-site"}},
	}
	for _, c := range refused {
		if got := request(s, c.method, base(s)+"done", c.header).Code; got != http.StatusForbidden && got != http.StatusNotFound {
			t.Errorf("%s %v: %d, want refused", c.method, c.header, got)
		}
	}
	select {
	case <-s.Done():
		t.Fatal("a refused request finished the page")
	default:
	}
	if got := request(s, http.MethodPost, base(s)+"done", map[string]string{"Origin": origin, "Sec-Fetch-Site": "same-origin"}).Code; got != http.StatusNoContent {
		t.Fatalf("the page's own Done: %d", got)
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("Done did not finish the page")
	}
	// A second click is harmless.
	if got := request(s, http.MethodPost, base(s)+"done", map[string]string{"Origin": origin}).Code; got != http.StatusNoContent {
		t.Fatalf("a second Done: %d", got)
	}
}

func TestThePageChecksIn(t *testing.T) {
	s, _ := newServer(t, time.Minute, time.Minute)
	s.seen = time.Now().Add(-time.Hour)
	if got := request(s, http.MethodGet, base(s)+"alive", nil).Code; got != http.StatusNoContent {
		t.Fatalf("alive: %d", got)
	}
	if time.Since(s.seen) > time.Minute {
		t.Error("checking in did not count")
	}
	if s.loaded {
		t.Error("checking in counted as the page loading")
	}
}

func TestServeEndsOnDone(t *testing.T) {
	s, listener := newServer(t, time.Hour, time.Hour)
	served := serve(t, s, listener, context.Background())
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(s.URL())
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "MySetup scan") {
		t.Fatalf("page over the network: %d", response.StatusCode)
	}
	done, _ := http.NewRequest(http.MethodPost, s.URL()+"done", nil)
	done.Header.Set("Origin", "http://"+s.host)
	response, err = client.Do(done)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	ended(t, served, 3*time.Second)
}

func TestServeEndsWhenTheTabHasGone(t *testing.T) {
	s, listener := newServer(t, time.Hour, time.Millisecond)
	request(s, http.MethodGet, base(s), nil)
	ended(t, serve(t, s, listener, context.Background()), 3*time.Second)
}

func TestServeWaitsForAPageThatHasNotLoaded(t *testing.T) {
	s, listener := newServer(t, time.Hour, time.Millisecond)
	served := serve(t, s, listener, context.Background())
	select {
	case <-served:
		t.Fatal("the app ended before its page had loaded")
	case <-time.After(1500 * time.Millisecond):
	}
}

func TestServeEndsWhenItsTimeIsUp(t *testing.T) {
	s, listener := newServer(t, time.Millisecond, time.Hour)
	ended(t, serve(t, s, listener, context.Background()), 3*time.Second)
}

func TestServeEndsOnInterrupt(t *testing.T) {
	s, listener := newServer(t, time.Hour, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	served := serve(t, s, listener, ctx)
	cancel()
	ended(t, served, 3*time.Second)
}

func serve(t *testing.T, s *Server, listener net.Listener, ctx context.Context) chan error {
	t.Helper()
	served := make(chan error, 1)
	go func() { served <- s.Serve(ctx, listener) }()

	return served
}

func ended(t *testing.T, served chan error, within time.Duration) {
	t.Helper()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(within):
		t.Fatal("Serve did not end")
	}
}

type fakeListener struct{ addr net.Addr }

func (fakeListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (fakeListener) Close() error              { return nil }
func (l fakeListener) Addr() net.Addr          { return l.addr }
