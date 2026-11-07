package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eddinn/mysetup-scanner/internal/scan"
)

// Only the paths that end before any hardware is read: the readers have tests of their own, and the app is run on every system
// by the workflow.
func TestFlags(t *testing.T) {
	cases := []struct {
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{[]string{"--version"}, 0, "mysetup-scan " + scan.FullVersion() + "\n", ""},
		{[]string{"--help"}, 0, "--cli         print the report in this terminal instead", ""},
		{[]string{"-h"}, 0, "--no-browser", ""},
		{[]string{"--gui"}, 2, "", "Usage: mysetup-scan [--cli | --no-browser]"},
		{[]string{"cli"}, 2, "", "Usage:"},
		{[]string{"--cli", "--no-browser"}, 2, "", "--cli prints the report in the terminal, so it takes no --no-browser."},
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		code := run(c.args, &stdout, &stderr)
		if code != c.code || !strings.Contains(stdout.String(), c.stdout) || !strings.Contains(stderr.String(), c.stderr) {
			t.Errorf("%v: code %d, stdout %q, stderr %q", c.args, code, stdout.String(), stderr.String())
		}
		if c.stdout == "" && stdout.Len() > 0 {
			t.Errorf("%v wrote to stdout: %q", c.args, stdout.String())
		}
	}
}
