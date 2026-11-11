//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The desktop's own tools are taken from the system's folders only, never PATH, and tried in turn: the first that works shows the
// message. With none, or as root, nothing is started.
func TestTellUsesTheFirstSystemDialogThatWorks(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "shown")
	write := func(folder, name, body string) {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(dir, "zenity", "exit 1")
	write(dir, "kdialog", `printf '%s|' "$@" > "`+out+`"`)
	write(dir, "notify-send", `echo notify > "`+out+`"`)
	onPath := t.TempDir()
	write(onPath, "zenity", `echo from-path > "`+out+`"`)
	t.Setenv("PATH", onPath)
	dirs, root := dialogDirs, asRoot
	t.Cleanup(func() { dialogDirs, asRoot = dirs, root })
	dialogDirs = []string{t.TempDir(), dir}
	asRoot = func() bool { return false }

	tell("The page could not start.")
	shown, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(shown), "--error|The page could not start.|") {
		t.Fatalf("kdialog was not used after zenity failed: %q %v", shown, err)
	}

	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}
	asRoot = func() bool { return true }
	tell("Not as root.")
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a dialog was started as root: %v", err)
	}

	asRoot = func() bool { return false }
	dialogDirs = []string{t.TempDir()}
	tell("Nothing to show it with.")
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a tool on PATH was used: %v", err)
	}
}
