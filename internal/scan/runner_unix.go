//go:build unix

package scan

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// The system's own directories. A tool is run only from here, never found through PATH, so nothing the member's environment
// puts first can stand in for it.
var systemDirectories = []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin"}

const maxToolOutput = 4 * 1024 * 1024

// runTool runs one of the system's own tools with fixed arguments and returns what it printed, at most maxToolOutput, within
// 20 seconds. Nothing is passed to a shell. A tool the system lacks gives nothing.
func runTool(tool string, args ...string) []byte {
	var path string
	for _, dir := range systemDirectories {
		candidate := filepath.Join(dir, tool)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			path = candidate

			break
		}
	}
	if path == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	cmd.Stdin = nil
	var out bytes.Buffer
	cmd.Stdout = &limited{w: &out, left: maxToolOutput}
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		return nil
	}

	return out.Bytes()
}

type limited struct {
	w    io.Writer
	left int
}

func (l *limited) Write(p []byte) (int, error) {
	n := len(p)
	if l.left <= 0 {
		return n, nil
	}
	if len(p) > l.left {
		p = p[:l.left]
	}
	l.left -= len(p)
	_, err := l.w.Write(p)

	return n, err
}
