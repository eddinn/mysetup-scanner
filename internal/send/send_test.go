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
)

const report = "mysetup-report 1\nsource=app/0.2.0\nmysetup-report-end\n"

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

func TestTheReportGoesWithTheCodeAndTheReviewLinkComesBack(t *testing.T) {
	server, got := site(t, http.StatusCreated, `{"review_url": "SITE/setups/01M7/components/scan/01M8"}`)
	link, err := post(context.Background(), server.URL+"/api/scanner/reports", " k7qd m2xp ", report, "0.2.0")
	if err != nil || link != server.URL+"/setups/01M7/components/scan/01M8" {
		t.Fatalf("link %q, error %v", link, err)
	}
	want := map[string]string{"method": "POST", "path": "/api/scanner/reports", "type": "application/json", "accept": "application/json",
		"agent": "MySetup-Scanner/0.2.0", "code": "k7qd m2xp", "report": report}
	for key, value := range want {
		if (*got)[key] != value {
			t.Errorf("%s: %q, want %q", key, (*got)[key], value)
		}
	}
}

func TestTheSitesRefusalIsShownAsItSaysIt(t *testing.T) {
	server, _ := site(t, http.StatusUnprocessableEntity, `{"message": "This code is wrong, was already used or has expired."}`)
	_, err := post(context.Background(), server.URL+"/api/scanner/reports", "K7QD-M2XP", report, "0.2.0")
	var refused *Refused
	if !errors.As(err, &refused) || refused.Message != "This code is wrong, was already used or has expired." {
		t.Fatalf("error %v", err)
	}
}

func TestEveryOtherAnswerIsAnErrorWithNoLink(t *testing.T) {
	cases := []struct {
		status  int
		answer  string
		refused bool
		says    string
	}{
		{http.StatusTooManyRequests, `{}`, true, "Too many tries"},
		{http.StatusNotFound, `{"message": "Not found."}`, false, "not open on MySetup yet"},
		{http.StatusRequestEntityTooLarge, ``, false, "too large"},
		{http.StatusInternalServerError, `oops`, false, "HTTP 500"},
		{http.StatusFound, ``, false, "HTTP 302"},
		// A link to another site, or to anything but a build, is not shown.
		{http.StatusCreated, `{"review_url": "https://example.com/setups/1"}`, false, "unexpected address"},
		{http.StatusCreated, `{"review_url": "SITE/login"}`, false, "unexpected address"},
		{http.StatusCreated, `{"review_url": "SITE/setups/../login"}`, false, "unexpected address"},
		{http.StatusCreated, `{"review_url": "SITE/setups/%2e%2e/login"}`, false, "unexpected address"},
		{http.StatusCreated, `{"review_url": "SITE/setups/1%2Flogin"}`, false, "unexpected address"},
		{http.StatusCreated, `not json`, false, "could not be read"},
	}
	for _, c := range cases {
		server, _ := site(t, c.status, c.answer)
		link, err := post(context.Background(), server.URL+"/api/scanner/reports", "K7QD-M2XP", report, "0.2.0")
		var refused *Refused
		if link != "" || err == nil || errors.As(err, &refused) != c.refused || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%d %s: link %q, error %v", c.status, c.answer, link, err)
		}
	}
}

func TestARedirectIsNotFollowed(t *testing.T) {
	elsewhere, got := site(t, http.StatusCreated, `{"review_url": "SITE/setups/1"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/api/scanner/reports", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	if _, err := post(context.Background(), server.URL+"/api/scanner/reports", "K7QD-M2XP", report, "0.2.0"); err == nil {
		t.Fatal("a redirect was taken as success")
	}
	if (*got)["report"] != "" {
		t.Fatal("the report went where the redirect pointed")
	}
}

func TestSomethingThatIsNoCodeIsNotSent(t *testing.T) {
	server, got := site(t, http.StatusCreated, `{"review_url": "SITE/setups/1"}`)
	for _, code := range []string{"", "K7QD", "K7QD-M2XP-9", "K7QD-M2X!", "К7QD-M2XP"} {
		_, err := post(context.Background(), server.URL+"/api/scanner/reports", code, report, "0.2.0")
		var refused *Refused
		if !errors.As(err, &refused) {
			t.Errorf("%q: error %v", code, err)
		}
	}
	if len(*got) != 0 {
		t.Fatal("something that is no code was sent")
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
