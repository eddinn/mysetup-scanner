package send

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const report = "mysetup-report 1\nsource=app/0.3.0\nmysetup-report-end\n"

// site stands in for MySetup's upload route: it records what came, and answers as told.
func site(t *testing.T, status int, answer string) (*httptest.Server, *map[string]string) {
	t.Helper()
	got := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		got["method"], got["path"], got["type"], got["accept"], got["agent"] = r.Method, r.URL.Path, r.Header.Get("Content-Type"),
			r.Header.Get("Accept"), r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, strings.ReplaceAll(answer, "SITE", "http://"+r.Host))
	}))
	t.Cleanup(server.Close)

	return server, &got
}

const token = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCdE"

func TestTheReportGoesWithoutACodeAndTheOneTimeLinkComesBack(t *testing.T) {
	server, got := site(t, http.StatusCreated, `{"claim_url": "SITE/scan/claim/`+token+`", "expires_at": "2026-11-24T23:30:00+01:00"}`)
	link, err := post(context.Background(), server.URL+"/api/scanner/reports", report, "0.3.0")
	if err != nil || link.URL != server.URL+"/scan/claim/"+token || !link.Expires.Equal(time.Date(2026, 11, 24, 22, 30, 0, 0, time.UTC)) {
		t.Fatalf("link %+v, error %v", link, err)
	}
	want := map[string]string{"method": "POST", "path": "/api/scanner/reports", "type": "application/json", "accept": "application/json",
		"agent": "MySetup-Scanner/0.3.0", "report": report}
	for key, value := range want {
		if (*got)[key] != value {
			t.Errorf("%s: %q, want %q", key, (*got)[key], value)
		}
	}
	if _, sent := (*got)["code"]; sent {
		t.Error("a code was sent")
	}
}

func TestTheSitesRefusalIsShownAsItSaysIt(t *testing.T) {
	for _, status := range []int{http.StatusUnprocessableEntity, http.StatusServiceUnavailable} {
		server, _ := site(t, status, `{"message": "Sending is busy right now."}`)
		_, err := post(context.Background(), server.URL+"/api/scanner/reports", report, "0.3.0")
		var refused *Refused
		if !errors.As(err, &refused) || refused.Message != "Sending is busy right now." {
			t.Fatalf("%d: error %v", status, err)
		}
	}
}

func TestEveryOtherAnswerIsAnErrorWithNoLink(t *testing.T) {
	cases := []struct {
		status  int
		answer  string
		refused bool
		says    string
	}{
		{http.StatusTooManyRequests, `{}`, true, "Too many tries from your network. Try again in half an hour"},
		{http.StatusServiceUnavailable, `oops`, true, "Try again in a few minutes"},
		{http.StatusNotFound, `{"message": "Not found."}`, false, "not open on MySetup yet"},
		{http.StatusRequestEntityTooLarge, ``, false, "too large"},
		{http.StatusInternalServerError, `oops`, false, "HTTP 500"},
		{http.StatusFound, ``, false, "HTTP 302"},
		// A link to another site, or to anything but the one-time link as the site makes it, is not opened.
		{http.StatusCreated, `{"claim_url": "https://example.com/scan/claim/` + token + `"}`, false, "unexpected address"},
		{http.StatusCreated, `{"claim_url": "SITE/login"}`, false, "unexpected address"},
		{http.StatusCreated, `{"claim_url": "SITE/scan/claim/short"}`, false, "unexpected address"},
		{http.StatusCreated, `{"claim_url": "SITE/scan/claim/` + token + `/../../login"}`, false, "unexpected address"},
		{http.StatusCreated, `{"claim_url": "SITE/scan/claim/` + token + `?next=https://example.com"}`, false, "unexpected address"},
		{http.StatusCreated, `{"claim_url": "SITE/scan/claim/` + token + `#x"}`, false, "unexpected address"},
		{http.StatusCreated, `{"claim_url": "SITE/scan/claim/%41` + token[1:] + `"}`, false, "unexpected address"},
		{http.StatusCreated, `{"review_url": "SITE/setups/1"}`, false, "unexpected address"},
		{http.StatusCreated, `not json`, false, "could not be read"},
	}
	for _, c := range cases {
		server, _ := site(t, c.status, c.answer)
		link, err := post(context.Background(), server.URL+"/api/scanner/reports", report, "0.3.0")
		var refused *Refused
		if link.URL != "" || err == nil || errors.As(err, &refused) != c.refused || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%d %s: link %q, error %v", c.status, c.answer, link, err)
		}
	}
}

func TestTooManyTriesSaysWhenFromRetryAfter(t *testing.T) {
	for header, says := range map[string]string{"1500": "Try again in 25 minutes,", "30": "Try again in a minute,", "0": "Try again now,", "soon": "Try again in half an hour,"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", header)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"message": "Too Many Attempts."}`)
		}))
		_, err := post(context.Background(), server.URL+"/api/scanner/reports", report, "0.3.0")
		server.Close()
		if err == nil || !strings.Contains(err.Error(), says) || !strings.Contains(err.Error(), "or copy and paste the report instead") {
			t.Errorf("Retry-After %s: %v", header, err)
		}
	}
}

func TestOnlyTheLinkAsMySetupMakesItIsTaken(t *testing.T) {
	const endpoint = "https://mysetup.is/api/scanner/reports"
	if got, err := claimLink(endpoint, "https://mysetup.is/scan/claim/"+token+"?"); err != nil || got != "https://mysetup.is/scan/claim/"+token {
		t.Errorf("the link itself: %q, %v", got, err)
	}
	for _, link := range []string{
		"http://mysetup.is/scan/claim/" + token, "https://mysetup.is:443/scan/claim/" + token, "https://MYSETUP.IS/scan/claim/" + token,
		"https://mysetup.is./scan/claim/" + token, "https://user@mysetup.is/scan/claim/" + token, "https://mysetup.is.example.com/scan/claim/" + token,
		"//mysetup.is/scan/claim/" + token, "/scan/claim/" + token, "javascript:alert(1)", "https://mysetup.is/scan/claim/" + token + "/x",
		"https://mysetup.is/scan/claim/" + token[:42], "https://mysetup.is/scan/claim/" + token + "%2F", " https://mysetup.is/scan/claim/" + token,
	} {
		if got, err := claimLink(endpoint, link); err == nil {
			t.Errorf("%q was taken as %q", link, got)
		}
	}
}

func TestARedirectIsNotFollowed(t *testing.T) {
	elsewhere, got := site(t, http.StatusCreated, `{"claim_url": "SITE/scan/claim/`+token+`"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/api/scanner/reports", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	if _, err := post(context.Background(), server.URL+"/api/scanner/reports", report, "0.3.0"); err == nil {
		t.Fatal("a redirect was taken as success")
	}
	if (*got)["report"] != "" {
		t.Fatal("the report went where the redirect pointed")
	}
}

func TestAReleaseBuildSendsOnlyToMySetup(t *testing.T) {
	if Development {
		t.Skip("a development build")
	}
	if Site() != "https://mysetup.is" || Endpoint() != "https://mysetup.is/api/scanner/reports" {
		t.Fatalf("endpoint %q", Endpoint())
	}
}
