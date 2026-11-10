//go:build windows

package page

import (
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Session is where the page opens. On Windows there is always a desktop. Run as administrator, the app first starts itself again
// as the signed-in user without administrator rights (the command's unelevate), so the page and the browser run as that user.
type Session struct{}

// FindSession: on Windows, the signed-in user's desktop.
func FindSession() (*Session, string) { return &Session{}, "" }

// Leave does nothing on Windows: an elevated app cannot give up its rights in place, so it starts again without them instead.
func (*Session) Leave() error { return nil }

// Open opens the page in the member's default browser, as Windows opens any web address. Still running as administrator (when
// starting again without it failed), it hands the address to Explorer, which opens it as the signed-in user, so the browser does
// not run as administrator.
func (*Session) Open(url string) error {
	if windows.GetCurrentProcessToken().IsElevated() {
		if folder, err := windows.GetWindowsDirectory(); err == nil {
			if err := exec.Command(filepath.Join(folder, "explorer.exe"), url).Start(); err == nil {
				return nil
			}
		}
	}
	verb, _ := windows.UTF16PtrFromString("open")
	target, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}

	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}
