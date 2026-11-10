//go:build linux

package scan

import (
	"os"

	"golang.org/x/sys/unix"
)

// Read reads this computer.
func Read() Hardware {
	var name unix.Utsname
	_ = unix.Uname(&name)
	h := ReadLinux(LinuxSource{FS: os.DirFS("/"), Root: os.Geteuid() == 0, Machine: unix.ByteSliceToString(name.Machine[:])})
	h.FinalNotes()

	return h
}
