//go:build unix

package page

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const (
	noDesktop = "There is no desktop here to open a browser on (over SSH, for example), so here is the report in the terminal instead."
	noOpener  = "This system has no xdg-open to open a browser with, so here is the report in the terminal instead."
)

// openerDirs are the only places the system's browser opener is taken from, never PATH.
var openerDirs = []string{"/usr/bin", "/usr/local/bin", "/bin"}

// Session is where the page opens: a desktop user's browser, with that user's own environment. Run as root (through sudo, doas,
// su or pkexec), the app reads the hardware first and then becomes the person who ran it (Leave) before it serves its page or
// opens a browser, so neither the page nor the browser ever runs as root. Only a desktop that is root's own opens as root.
type Session struct {
	UID, GID int
	Groups   []int
	Env      []string
	opener   string
}

// FindSession finds the desktop session to open the page in, or says why there is none: no desktop first, then no opener.
func FindSession() (*Session, string) {
	session := desktopSession()
	if session == nil {
		return nil, noDesktop
	}
	if session.opener = findOpener(); session.opener == "" {
		return nil, noOpener
	}

	return session, ""
}

func desktopSession() *Session {
	if os.Geteuid() != 0 {
		env := os.Environ()
		if !hasDisplay(env) {
			return nil
		}

		return &Session{UID: os.Getuid(), GID: os.Getgid(), Env: env}
	}
	// Run as root: the person who ran it, found through the processes it was started from (Linux), or through sudo's and doas's
	// own variables. A person without a desktop has none to open a page in, and nobody else's is taken.
	session, someone := processSession()
	if session == nil && !someone {
		session, someone = elevatorSession()
	}
	if session != nil || someone {
		return session
	}
	// Nobody else: a desktop that is root's own.
	if env := os.Environ(); hasDisplay(env) {
		return &Session{UID: 0, GID: 0, Env: env}
	}

	return nil
}

// Leave gives up root for the session's user, for good, before the page is served. It refuses to go on if root could be taken back.
func (s *Session) Leave() error {
	if os.Geteuid() != 0 || s.UID == 0 {
		return nil
	}
	groups := s.Groups
	if groups == nil {
		groups = []int{}
	}
	if err := syscall.Setgroups(groups); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setgid(s.GID); err != nil {
		return fmt.Errorf("setgid: %w", err)
	}
	if err := syscall.Setuid(s.UID); err != nil {
		return fmt.Errorf("setuid: %w", err)
	}
	if syscall.Setuid(0) == nil || os.Geteuid() != s.UID || os.Getuid() != s.UID || os.Getegid() != s.GID {
		return errors.New("root was not given up")
	}

	return nil
}

// Open opens the page in the session's browser through the system's own opener.
func (s *Session) Open(url string) error {
	cmd := exec.Command(s.opener, url)
	cmd.Env = s.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()

	return nil
}

func findOpener() string {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	for _, dir := range openerDirs {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return path
		}
	}

	return ""
}

// hasDisplay says whether an environment belongs to a desktop. A Mac's always does.
func hasDisplay(env []string) bool {
	if runtime.GOOS == "darwin" {
		return true
	}
	for _, kv := range env {
		if (strings.HasPrefix(kv, "DISPLAY=") || strings.HasPrefix(kv, "WAYLAND_DISPLAY=")) && !strings.HasSuffix(kv, "=") {
			return true
		}
	}

	return false
}

// elevatorSession is the person sudo or doas ran the app for, from the variables they set, with the environment root was given
// (sudo keeps the display on some systems) and that person's own home and name. someone is true when such a person exists.
func elevatorSession() (session *Session, someone bool) {
	var account *user.User
	if id := os.Getenv("SUDO_UID"); id != "" && id != "0" {
		account, _ = user.LookupId(id)
	} else if name := os.Getenv("DOAS_USER"); name != "" && name != "root" {
		account, _ = user.Lookup(name)
	}
	if account == nil {
		return nil, false
	}
	uid, errUID := strconv.Atoi(account.Uid)
	gid, errGID := strconv.Atoi(account.Gid)
	if errUID != nil || errGID != nil || uid == 0 {
		return nil, false
	}
	var groups []int
	if ids, err := account.GroupIds(); err == nil {
		for _, id := range ids {
			if n, err := strconv.Atoi(id); err == nil {
				groups = append(groups, n)
			}
		}
	}
	env := []string{"HOME=" + account.HomeDir, "USER=" + account.Username, "LOGNAME=" + account.Username}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "HOME", "USER", "LOGNAME", "MAIL", "SUDO_COMMAND", "SUDO_USER", "SUDO_UID", "SUDO_GID", "DOAS_USER":
		default:
			env = append(env, kv)
		}
	}
	if !hasDisplay(env) {
		return nil, true
	}

	return &Session{UID: uid, GID: gid, Groups: groups, Env: env}, true
}
