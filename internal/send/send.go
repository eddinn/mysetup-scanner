// Package send hands the report to MySetup when the member clicks Send to MySetup (MySetup's docs/scanner/design.md, One tab; the
// site's POST /api/scanner/reports, without a code since 0.3.0). It is the only part of the app that uses the network, and only
// after the member asked it to send. It sends the report the member read, to an address fixed in a release build (site_release.go;
// staging only in a development build, site_dev.go), follows no redirect, reads a bounded answer and takes back only a one-time
// link on the same site, where the member signs in and chooses the build.
package send

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	timeout   = 30 * time.Second
	maxAnswer = 64 * 1024
	// maxMessage bounds what of the site's message the app shows.
	maxMessage = 300
)

// Refused is the site's answer for the member to read: a report it does not take, too many tries, too many reports waiting. The
// member can try again, or copy and paste the report instead.
type Refused struct{ Message string }

func (r *Refused) Error() string { return r.Message }

var client = &http.Client{
	Timeout: timeout,
	// A redirect could carry the report somewhere else: none is followed.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Report sends the report to MySetup and returns the one-time link that adds it to one of the member's builds there.
func Report(ctx context.Context, report, version string) (string, error) {
	return post(ctx, Endpoint(), report, version)
}

// Endpoint is the only address the app sends to: the upload route of Site.
func Endpoint() string { return Site() + "/api/scanner/reports" }

func post(ctx context.Context, endpoint, report, version string) (string, error) {
	body, err := json.Marshal(map[string]string{"report": report})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "MySetup-Scanner/"+version)

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("MySetup could not be reached (%s)", plain(err.Error()))
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(response.Body, maxAnswer))
	if err != nil {
		return "", fmt.Errorf("MySetup's answer could not be read (%s)", plain(err.Error()))
	}

	switch response.StatusCode {
	case http.StatusCreated:
		var created struct {
			ClaimURL string `json:"claim_url"`
		}
		if json.Unmarshal(answer, &created) != nil {
			return "", errors.New("MySetup's answer could not be read")
		}

		return claimLink(endpoint, created.ClaimURL)
	case http.StatusUnprocessableEntity, http.StatusServiceUnavailable:
		// A report it does not take, or too many waiting for their members just now: the site says which.
		var refused struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(answer, &refused) != nil || plain(refused.Message) == "" {
			return "", &Refused{"MySetup did not take the report. Try again in a few minutes, or copy and paste it instead."}
		}

		return "", &Refused{plain(refused.Message)}
	case http.StatusTooManyRequests:
		return "", &Refused{"Too many tries from your network. Wait a minute, then try again."}
	case http.StatusRequestEntityTooLarge:
		return "", errors.New("MySetup says the report is too large to send. Copy and paste it instead")
	case http.StatusNotFound:
		return "", errors.New("sending reports is not open on MySetup yet. Copy and paste the report instead")
	default:
		return "", fmt.Errorf("MySetup answered with an error (HTTP %d). Try again later, or copy and paste the report instead", response.StatusCode)
	}
}

// claimPath is the only address the app opens on the site: the one-time link, a token of 43 URL-safe characters.
var claimPath = regexp.MustCompile(`\A/scan/claim/[A-Za-z0-9_-]{43}\z`)

// claimLink takes the link only on the site the report went to, and only as the site makes it: nothing else is opened.
func claimLink(endpoint, link string) (string, error) {
	site, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	claim, err := url.Parse(link)
	if err != nil || claim.Scheme != site.Scheme || claim.Host != site.Host || !claimPath.MatchString(claim.Path) || claim.RawPath != "" ||
		claim.RawQuery != "" || claim.User != nil || claim.Fragment != "" {
		return "", errors.New("MySetup's answer named an unexpected address, so the app does not open it")
	}

	return claim.String(), nil
}

// plain keeps a short line of printable text.
func plain(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}

		return ' '
	}, strings.TrimSpace(text))
	if len([]rune(text)) > maxMessage {
		text = string([]rune(text)[:maxMessage]) + "…"
	}

	return text
}
