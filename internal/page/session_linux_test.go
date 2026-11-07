package page

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func status(uid, gid, parent int, groups string) *fstest.MapFile {
	return statusOf(uid, uid, gid, parent, groups)
}

// statusOf is a process's status with its real and effective users apart, as sudo's: real the person, effective root.
func statusOf(real, effective, gid, parent int, groups string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("Name:\tx\nUmask:\t0022\nState:\tS (sleeping)\nTgid:\t1\nNgid:\t0\nPPid:\t" + strconv.Itoa(parent) +
		"\nUid:\t" + strconv.Itoa(real) + "\t" + strconv.Itoa(effective) + "\t" + strconv.Itoa(effective) + "\t" + strconv.Itoa(effective) +
		"\nGid:\t" + strconv.Itoa(gid) + "\t" + strconv.Itoa(gid) + "\t" + strconv.Itoa(gid) + "\t" + strconv.Itoa(gid) +
		"\nFDSize:\t64\nGroups:\t" + groups + "\n")}
}

func environ(vars ...string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(strings.Join(vars, "\x00") + "\x00")}
}

// The made-up processes of `sudo ./mysetup-scan` typed in a desktop's terminal, as the kernel shows them: the app's parent is
// sudo, with the person as its real user but root as its effective user and group, and the desktop in its environment; its
// parent is the person's shell.
func TestTheSessionIsThePersonWhoRanSudo(t *testing.T) {
	proc := fstest.MapFS{
		"400/status": status(0, 0, 300, ""), "400/environ": environ("DISPLAY=:0", "HOME=/root"),
		"300/status": statusOf(1000, 0, 0, 200, "0"), "300/environ": environ("DISPLAY=:0", "HOME=/home/a"),
		"200/status": status(1000, 1000, 100, "4 24 27 1000"), "200/environ": environ("DISPLAY=:0", "XAUTHORITY=/home/a/.Xauthority", "HOME=/home/a"),
		"100/status": status(1000, 1000, 1, "4 24 27 1000"), "100/environ": environ("HOME=/home/a"),
	}
	session, someone := sessionFromProc(proc, 400)
	if session == nil || !someone {
		t.Fatalf("no session: %v %v", session, someone)
	}
	if session.UID != 1000 || session.GID != 1000 || fmtInts(session.Groups) != "[4,24,27,1000]" {
		t.Errorf("session %d:%d %v", session.UID, session.GID, session.Groups)
	}
	if strings.Join(session.Env, " ") != "DISPLAY=:0 XAUTHORITY=/home/a/.Xauthority HOME=/home/a" {
		t.Errorf("environment %q, want the shell's own", session.Env)
	}
}

func TestAPersonWithoutADesktopHasNoSessionAndNobodyElseIsTried(t *testing.T) {
	proc := fstest.MapFS{
		"300/status": status(0, 0, 200, ""), "300/environ": environ("HOME=/root"),
		// Over SSH: the person's shell has no display. Another person further up must not be taken instead.
		"200/status": status(1000, 1000, 100, "1000"), "200/environ": environ("SSH_CONNECTION=x", "HOME=/home/a"),
		"100/status": status(1001, 1001, 1, "1001"), "100/environ": environ("DISPLAY=:1", "HOME=/home/b"),
	}
	if session, someone := sessionFromProc(proc, 300); session != nil || !someone {
		t.Fatalf("got %v %v, want nobody's desktop but someone", session, someone)
	}
}

func TestOnlyRootAboveMeansNoPerson(t *testing.T) {
	proc := fstest.MapFS{"300/status": status(0, 0, 200, ""), "200/status": status(0, 0, 1, "")}
	if session, someone := sessionFromProc(proc, 300); session != nil || someone {
		t.Fatalf("got %v %v, want no one", session, someone)
	}
	if session, someone := sessionFromProc(fstest.MapFS{"300/status": {Data: []byte("garbage")}}, 300); session != nil || someone {
		t.Fatalf("an unreadable status gave %v %v", session, someone)
	}
}

func TestStatusIsReadByItsOwnFields(t *testing.T) {
	uid, gid, groups, parent, rooted, ok := parseStatus(string(status(1000, 1001, 42, "4 27").Data))
	if !ok || rooted || uid != 1000 || gid != 1001 || parent != 42 || len(groups) != 2 || groups[1] != 27 {
		t.Fatalf("%d %d %v %d %v %v", uid, gid, groups, parent, rooted, ok)
	}
	if _, _, _, _, rooted, ok := parseStatus(string(statusOf(1000, 0, 0, 1, "0").Data)); !ok || !rooted {
		t.Fatal("sudo's status, the person's real user with root's effective one, did not count as root's")
	}
	if _, _, _, _, _, ok := parseStatus("Uid:\tx\nGid:\t1\nPPid:\t1\n"); ok {
		t.Fatal("a status with a broken Uid was read")
	}
}

func TestADisplayNeedsAValue(t *testing.T) {
	if hasDisplay([]string{"DISPLAY=", "WAYLAND_DISPLAY="}) || !hasDisplay([]string{"WAYLAND_DISPLAY=wayland-0"}) {
		t.Fatal("display read wrongly")
	}
}

func TestSudosVariablesNameThePersonWithTheirOwnHome(t *testing.T) {
	me := strconv.Itoa(os.Getuid())
	if me == "0" {
		t.Skip("run as root, sudo's variables cannot name root")
	}
	t.Setenv("SUDO_UID", me)
	t.Setenv("HOME", "/root")
	t.Setenv("DISPLAY", ":7")
	session, someone := elevatorSession()
	if session == nil || !someone || strconv.Itoa(session.UID) != me {
		t.Fatalf("got %v %v", session, someone)
	}
	joined := "\x00" + strings.Join(session.Env, "\x00") + "\x00"
	if strings.Contains(joined, "\x00HOME=/root\x00") || strings.Contains(joined, "SUDO_UID=") || !strings.Contains(joined, "\x00DISPLAY=:7\x00") {
		t.Errorf("environment %q", session.Env)
	}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	if session, someone := elevatorSession(); session != nil || !someone {
		t.Fatalf("without a display: %v %v", session, someone)
	}
}

func fmtInts(values []int) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}

	return "[" + strings.Join(parts, ",") + "]"
}

// Run as root from a desktop (go test -exec 'sudo -n env MYSETUP_ROOT_TEST=1' -run TestRoot ./internal/page/), a child process
// finds the person who ran sudo, gives up root for good, and opens a page through a stand-in opener, which records who it ran as.
func TestRootGivesUpRootAndOpensAsThePerson(t *testing.T) {
	if os.Geteuid() != 0 || os.Getenv("MYSETUP_ROOT_TEST") != "1" {
		t.Skip("needs root from a desktop: go test -exec 'sudo -n env MYSETUP_ROOT_TEST=1'")
	}
	dir, err := os.MkdirTemp("", "mysetup-root-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	mark := filepath.Join(dir, "opened")
	script := "#!/bin/sh\n{ id -u; id -g; id -G; printf '%s\\n' \"$DISPLAY$WAYLAND_DISPLAY\" \"$HOME\" \"$1\"; } > " + mark + "\n"
	if err := os.WriteFile(filepath.Join(dir, "xdg-open"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestRootHelper$", "-test.v")
	child.Env = append(os.Environ(), "MYSETUP_ROOT_HELPER="+dir)
	out, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	data, err := os.ReadFile(mark)
	if err != nil {
		t.Fatalf("the opener did not run: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	sudoUID := os.Getenv("SUDO_UID")
	rootGroup := false
	for _, group := range strings.Fields(lines[min(2, len(lines)-1)]) {
		rootGroup = rootGroup || group == "0"
	}
	if len(lines) != 6 || lines[0] != sudoUID || lines[0] == "0" || lines[1] == "0" || rootGroup || lines[3] == "" || lines[4] == "/root" ||
		lines[5] != "http://127.0.0.1:1/t/x/" {
		t.Fatalf("the opener ran as %q, want the person who ran sudo (%s) with their desktop and home\n%s", lines, sudoUID, out)
	}
	t.Logf("opened as uid %s gid %s groups %s, display %s, home %s", lines[0], lines[1], lines[2], lines[3], lines[4])
}

func TestRootHelper(t *testing.T) {
	dir := os.Getenv("MYSETUP_ROOT_HELPER")
	if dir == "" {
		t.Skip("only run by TestRootGivesUpRootAndOpensAsThePerson")
	}
	openerDirs = []string{dir}
	session, why := FindSession()
	if session == nil {
		t.Fatalf("no session: %s", why)
	}
	if err := session.Leave(); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if os.Geteuid() == 0 || os.Getuid() == 0 {
		t.Fatal("still root")
	}
	if err := os.WriteFile("/root/.mysetup-root-test", nil, 0o600); err == nil {
		_ = os.Remove("/root/.mysetup-root-test")
		t.Fatal("could still write as root")
	}
	if err := session.Open("http://127.0.0.1:1/t/x/"); err != nil {
		t.Fatalf("open: %v", err)
	}
	// The opener runs on its own; give it a moment to record.
	for range 50 {
		if _, err := os.Stat(filepath.Join(dir, "opened")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the opener did not record")
}
