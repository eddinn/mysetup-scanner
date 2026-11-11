//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// stderrIsTerminal says whether the app writes to a terminal, the way Go's own golang.org/x/term finds out.
func stderrIsTerminal() bool {
	_, err := unix.IoctlGetTermios(int(os.Stderr.Fd()), unix.TIOCGETA)

	return err == nil
}
