//go:build !windows

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// keepWindowOpen does nothing outside Windows: no console window is opened for the app there and closed when it ends.
func keepWindowOpen(string, io.Writer) {}

// detach does nothing outside Windows: no console window is opened for the app there.
func detach() bool { return false }

// unelevate does nothing outside Windows: run as root there, the app gives up root in place (page.Session.Leave).
func unelevate() bool { return false }

// leaveOwnConsole does nothing outside Windows.
func leaveOwnConsole() {}

// startedWithoutTerminal says whether the app was started with no terminal to write to, as by a double-click in a file
// manager. It then says what goes wrong in the desktop's own message box (tell), as it does on Windows.
func startedWithoutTerminal() bool { return !stderrIsTerminal() }

// dialogs are the desktop's own tools for a message, tried in turn: GNOME's zenity, KDE's kdialog, then a notification. None is
// required (the app needs nothing installed); with none, the message stays where it was written, as before.
var dialogs = [][]string{
	{"zenity", "--error", "--title=MySetup scan", "--no-wrap", "--text"},
	{"kdialog", "--title", "MySetup scan", "--error"},
	{"notify-send", "--urgency=critical", "MySetup scan"},
}

// dialogDirs are the only places a dialog tool is taken from, never PATH, as for the browser opener (internal/page).
var dialogDirs = []string{"/usr/bin", "/usr/local/bin", "/bin"}

// asRoot says whether the app still runs as root, which never starts a desktop tool: the message then stays in the error output.
var asRoot = func() bool { return os.Geteuid() == 0 }

// tell shows a message when the app has no terminal, with the first of the desktop's own tools that works.
func tell(message string) {
	if asRoot() {
		return
	}
	for _, dialog := range dialogs {
		path := systemTool(dialog[0])
		if path == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		err := exec.CommandContext(ctx, path, append(append([]string{}, dialog[1:]...), message)...).Run()
		cancel()
		if err == nil {
			return
		}
	}
}

// systemTool finds a tool in dialogDirs only: a regular, executable file, or "" when there is none.
func systemTool(name string) string {
	for _, dir := range dialogDirs {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return path
		}
	}

	return ""
}
