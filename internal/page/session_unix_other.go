//go:build unix && !linux

package page

// processSession: outside Linux the processes the app was started from are not read; sudo's and doas's own variables name the
// person instead (elevatorSession).
func processSession() (*Session, bool) { return nil, false }
