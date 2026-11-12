//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal says whether f is a terminal, the way Go's own golang.org/x/term finds out.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TIOCGETA)

	return err == nil
}

// stderrIsTerminal says whether the app writes to a terminal.
func stderrIsTerminal() bool { return isTerminal(os.Stderr) }

// canAsk says whether the app can ask the member for a code: it reads from a terminal and writes to one, so a member sees the
// question and can answer it. Run by a script or with its input redirected, it never waits for an answer nobody can give.
func canAsk() bool { return isTerminal(os.Stdin) && isTerminal(os.Stderr) }
