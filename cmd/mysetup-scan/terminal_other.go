//go:build !windows && !linux && !solaris && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package main

// stderrIsTerminal assumes a terminal where the app cannot ask: it keeps writing there, as before.
func stderrIsTerminal() bool { return true }
