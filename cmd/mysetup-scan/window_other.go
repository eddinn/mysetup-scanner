//go:build !windows

package main

import "io"

// keepWindowOpen does nothing outside Windows: no console window is opened for the app there and closed when it ends.
func keepWindowOpen(string, io.Writer) {}

// detach does nothing outside Windows: no console window is opened for the app there.
func detach() bool { return false }

// unelevate does nothing outside Windows: run as root there, the app gives up root in place (page.Session.Leave).
func unelevate() bool { return false }

// leaveOwnConsole does nothing outside Windows.
func leaveOwnConsole() {}

// tell does nothing outside Windows: only there does the app run without a terminal to write to (detach).
func tell(string) {}
