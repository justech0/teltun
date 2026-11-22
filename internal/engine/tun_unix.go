//go:build !windows

package engine

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"
)

// startTun creates the TelTun card. Creating a network card needs root, so
// unless we already are root, the same binary is started again as a small
// privileged helper (--tun-helper). The window and adb stay unprivileged.
func startTun(socksPort int, blockQUIC bool) (io.Closer, error) {
	if os.Geteuid() == 0 {
		return startBox(socksPort, blockQUIC)
	}
	return startHelper(socksPort, blockQUIC)
}

type helper struct {
	conn net.Conn
	cmd  *exec.Cmd
	done chan struct{}
}

// Close drops the control link; the helper removes the card and exits.
func (h *helper) Close() error {
	h.conn.Close()
	select {
	case <-h.done:
	case <-time.After(10 * time.Second):
	}
	return nil
}

func startHelper(socksPort int, blockQUIC bool) (io.Closer, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer ln.Close()

	tok := make([]byte, 16)
	rand.Read(tok)
	token := hex.EncodeToString(tok)
	args := []string{"--tun-helper", "--ctl", ln.Addr().String(), "--token", token,
		"--socks", strconv.Itoa(socksPort), "--log", DataDir()}
	if blockQUIC {
		args = append(args, "--block-quic")
	}

	cmd, err := elevate(exe, args)
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fail("errAuth", err)
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()

	// Wait for the helper to call back. The user may take a while to type
	// the password, so be patient; give up if the helper exits first.
	type accepted struct {
		c   net.Conn
		r   *bufio.Reader
		err error
	}
	acc := make(chan accepted, 1)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				acc <- accepted{err: err}
				return
			}
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			r := bufio.NewReader(c)
			line, _ := r.ReadString('\n')
			if strings.TrimSpace(line) == token {
				acc <- accepted{c: c, r: r}
				return
			}
			c.Close()
		}
	}()

	var conn net.Conn
	var rd *bufio.Reader
	select {
	case a := <-acc:
		if a.err != nil {
			return nil, a.err
		}
		conn, rd = a.c, a.r
	case <-done:
		return nil, fail("errAuth", errors.New("password prompt was cancelled"))
	case <-time.After(3 * time.Minute):
		cmd.Process.Kill()
		return nil, fail("errAuth", errors.New("no answer from the password prompt"))
	}

	// First answer: "ready" or "error: ...".
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	line, err := rd.ReadString('\n')
	conn.SetReadDeadline(time.Time{})
	line = strings.TrimSpace(line)
	if err != nil || line != "ready" {
		conn.Close()
		if msg, ok := strings.CutPrefix(line, "error: "); ok {
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("helper: %v %s", err, line)
	}
	return &helper{conn: conn, cmd: cmd, done: done}, nil
}

// elevate builds the command that runs exe as root with a password prompt.
func elevate(exe string, args []string) (*exec.Cmd, error) {
	if goruntime.GOOS == "darwin" {
		q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
		parts := []string{q(exe)}
		for _, a := range args {
			parts = append(parts, q(a))
		}
		script := fmt.Sprintf(`do shell script %q with administrator privileges`,
			strings.Join(parts, " ")+" >/dev/null 2>&1")
		return exec.Command("osascript", "-e", script), nil
	}
	pk, err := exec.LookPath("pkexec")
	if err != nil {
		return nil, fail("errNoPkexec", err)
	}
	return exec.Command(pk, append([]string{exe}, args...)...), nil
}
