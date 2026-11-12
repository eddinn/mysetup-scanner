//go:build dev

package send

import "testing"

func TestADevelopmentBuildSendsToStagingOrThisComputerOnly(t *testing.T) {
	cases := map[string]string{
		"":                            "https://staging.mysetup.is/api/scanner/reports",
		"https://staging.mysetup.is/": "https://staging.mysetup.is/api/scanner/reports",
		"http://127.0.0.1:8000":       "http://127.0.0.1:8000/api/scanner/reports",
		"https://mysetup.is":          "https://staging.mysetup.is/api/scanner/reports",
		"https://example.com":         "https://staging.mysetup.is/api/scanner/reports",
		"http://staging.mysetup.is":   "https://staging.mysetup.is/api/scanner/reports",
		"http://127.0.0.1:8000/x?y#z": "http://127.0.0.1:8000/api/scanner/reports",
		"http://a@localhost:8000":     "https://staging.mysetup.is/api/scanner/reports",
	}
	for site, want := range cases {
		t.Setenv("MYSETUP_SCAN_SITE", site)
		if got := Endpoint(); got != want || got != Site()+"/api/scanner/reports" {
			t.Errorf("%q: %q, want %q", site, got, want)
		}
	}
}
