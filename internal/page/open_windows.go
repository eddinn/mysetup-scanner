//go:build windows

package page

import (
	"golang.org/x/sys/windows"
)

// Desktop says whether a browser can be opened here, and why not. Run as administrator, the app does not open a browser: a
// browser started from it would run as administrator too. On Windows nothing in the report needs administrator anyway.
func Desktop() (bool, string) {
	if windows.GetCurrentProcessToken().IsElevated() {
		return false, "Run as administrator, the app does not open a browser, and Windows needs no administrator for the report; here it is instead."
	}

	return true, ""
}

// Open opens the page in the member's default browser, as Windows opens any web address.
func Open(url string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	target, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}

	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}
