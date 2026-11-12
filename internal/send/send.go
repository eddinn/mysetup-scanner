// Package send hands the report to the member's build on MySetup with a code from the build page (MySetup's docs/scanner/design.md,
// Pairing; the site's POST /api/scanner/reports). It is the only part of the app that uses the network, and only after the member
// typed a code. It sends the report the member read, to an address fixed in a release build (site_release.go; staging only in a
// development build, site_dev.go), follows no redirect, reads a bounded answer and takes a review link only on the same site.
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
	"path"
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

// Refused is the site's answer for the member to read: a wrong, used or expired code, a report it does not take, too many tries.
// The member can try again with another code.
type Refused struct{ Message string }

func (r *Refused) Error() string { return r.Message }

var client = &http.Client{
	Timeout: timeout,
	// A redirect could carry the report somewhere else: none is followed.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Report sends the report with the code to MySetup and returns the address of its review on the member's build.
func Report(ctx context.Context, code, report, version string) (string, error) {
	return post(ctx, Endpoint(), code, report, version)
}

// Endpoint is the only address the app sends to: the upload route of Site.
func Endpoint() string { return Site() + "/api/scanner/reports" }

func post(ctx context.Context, endpoint, code, report, version string) (string, error) {
	code = strings.TrimSpace(code)
	if !looksLikeCode(code) {
		return "", &Refused{"A code has 8 letters and digits, like K7QD-M2XP. Get one on your build page, then type it here."}
	}
	body, err := json.Marshal(map[string]string{"code": code, "report": report})
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
			ReviewURL string `json:"review_url"`
		}
		if json.Unmarshal(answer, &created) != nil {
			return "", errors.New("MySetup's answer could not be read")
		}

		return reviewLink(endpoint, created.ReviewURL)
	case http.StatusUnprocessableEntity:
		var refused struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(answer, &refused) != nil || plain(refused.Message) == "" {
			return "", &Refused{"MySetup did not take the report. Get a new code on your build page and try again."}
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

// looksLikeCode says whether what was typed can be a code: 8 letters and digits, with any spaces and dashes. The site reads it.
func looksLikeCode(code string) bool {
	count := 0
	for _, r := range code {
		switch {
		case r == ' ' || r == '-':
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			count++
		default:
			return false
		}
	}

	return count == 8
}

// reviewLink takes the review's address only on the site the report went to, and only a build's page there.
func reviewLink(endpoint, link string) (string, error) {
	site, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	review, err := url.Parse(link)
	if err != nil || review.Scheme != site.Scheme || review.Host != site.Host || !strings.HasPrefix(review.Path, "/setups/") ||
		path.Clean(review.Path) != review.Path || review.RawPath != "" || review.User != nil || review.Fragment != "" {
		return "", errors.New("MySetup's answer named an unexpected address, so the app does not show it")
	}

	return review.String(), nil
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
