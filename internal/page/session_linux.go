//go:build linux

package page

import (
	"bytes"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// processSession finds, among the processes the app was started from, the first that holds no root at all: the person who ran
// sudo, doas, su or pkexec. sudo itself, and every program like it, keeps the person's real user beside root's effective one,
// with root's group, so a process counts only when none of its users is root. Its desktop session is the one its own environment
// names, with its own user, group and groups as the kernel gives them. someone is true when such a process exists, with or
// without a desktop.
func processSession() (session *Session, someone bool) {
	return sessionFromProc(os.DirFS("/proc"), os.Getppid())
}

func sessionFromProc(proc fs.FS, pid int) (*Session, bool) {
	for range 64 {
		if pid <= 1 {
			return nil, false
		}
		status, err := fs.ReadFile(proc, strconv.Itoa(pid)+"/status")
		if err != nil {
			return nil, false
		}
		uid, gid, groups, parent, rooted, ok := parseStatus(string(status))
		if !ok {
			return nil, false
		}
		if !rooted {
			environ, err := fs.ReadFile(proc, strconv.Itoa(pid)+"/environ")
			if err != nil {
				return nil, true
			}
			var env []string
			for _, kv := range bytes.Split(environ, []byte{0}) {
				if bytes.IndexByte(kv, '=') > 0 {
					env = append(env, string(kv))
				}
			}
			if !hasDisplay(env) {
				return nil, true
			}

			return &Session{UID: uid, GID: gid, Groups: groups, Env: env}, true
		}
		pid = parent
	}

	return nil, false
}

// parseStatus reads a process's real user and group, its groups and its parent from /proc/PID/status, and whether any of its
// users (real, effective, saved, filesystem) is root.
func parseStatus(status string) (uid, gid int, groups []int, parent int, rooted, ok bool) {
	seen := 0
	for _, line := range strings.Split(status, "\n") {
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(value)
		switch name {
		case "Uid", "Gid", "PPid":
			if len(fields) == 0 {
				return 0, 0, nil, 0, false, false
			}
			n, err := strconv.Atoi(fields[0])
			if err != nil || n < 0 {
				return 0, 0, nil, 0, false, false
			}
			switch name {
			case "Uid":
				uid = n
				for _, field := range fields {
					if id, err := strconv.Atoi(field); err != nil || id == 0 {
						rooted = true
					}
				}
			case "Gid":
				gid = n
			default:
				parent = n
			}
			seen++
		case "Groups":
			for _, field := range fields {
				if n, err := strconv.Atoi(field); err == nil && n >= 0 {
					groups = append(groups, n)
				}
			}
		}
	}

	return uid, gid, groups, parent, rooted, seen == 3
}
