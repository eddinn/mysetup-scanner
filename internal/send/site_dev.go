//go:build dev

package send

import (
	"net/url"
	"os"
	"strings"
)

// Site is the MySetup a development build (go build -tags dev) sends to and leads the member to: MySetup's staging site, or the
// site MYSETUP_SCAN_SITE names when it is staging or this computer, for a test. A release build cannot send there
// (site_release.go).
func Site() string {
	const staging = "https://staging.mysetup.is"
	site := strings.TrimRight(os.Getenv("MYSETUP_SCAN_SITE"), "/")
	if site == staging {
		return staging
	}
	if parsed, err := url.Parse(site); err == nil && parsed.Scheme == "http" && parsed.User == nil &&
		(parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost") {
		return "http://" + parsed.Host
	}

	return staging
}

// Development says whether this is a development build, which may send to staging.
const Development = true
