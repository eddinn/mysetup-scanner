//go:build unix && !linux && !darwin && !freebsd && !openbsd && !netbsd

package scan

import (
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

// Read reads this computer: illumos through its own tools, any other system as far as it lets the app.
func Read() Hardware {
	var name unix.Utsname
	_ = unix.Uname(&name)
	release := unix.ByteSliceToString(name.Release[:])
	src := UnixSource{Run: runTool, Root: os.Geteuid() == 0, Machine: unix.ByteSliceToString(name.Machine[:]),
		Sysctl: func(key string) string {
			if key == "kern.osrelease" {
				return release
			}

			return ""
		}}
	var h Hardware
	if runtime.GOOS == "illumos" || runtime.GOOS == "solaris" {
		h = ReadIllumos(src)
	} else {
		h = Hardware{Family: "other", Architecture: NormalArchitecture(src.Machine), Environment: "unknown"}
		h.CPU.Model = keyValue(readFile(os.DirFS("/"), "proc/cpuinfo"), ":", "model name")
		h.OSName = strings.ToLower(runtime.GOOS)
	}
	h.FinalNotes()

	return h
}
