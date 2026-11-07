//go:build unix

package page

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Desktop says whether a browser can be opened here, and why not. Run as root, the app does not open a browser, since a browser
// started from it would run as root too; it prints the report in the terminal instead.
func Desktop() (bool, string) {
	if os.Geteuid() == 0 {
		return false, "Run as root, the app does not open a browser; here is the report instead."
	}
	if runtime.GOOS == "darwin" {
		return true, ""
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false, "There is no desktop here for a browser, so here is the report instead."
	}
	if opener() == "" {
		return false, "This system has no xdg-open to open a browser with, so here is the report instead."
	}

	return true, ""
}

// Open opens the page in the member's default browser through the system's own opener, found in the system's directories only.
func Open(url string) error {
	path := opener()
	if path == "" {
		return errors.New("no opener")
	}
	cmd := exec.Command(path, url)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()

	return nil
}

func opener() string {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	for _, dir := range []string{"/usr/bin", "/usr/local/bin", "/bin"} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return path
		}
	}

	return ""
}
