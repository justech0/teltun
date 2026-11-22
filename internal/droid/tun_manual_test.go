//go:build manual && linux

// Manual end-to-end test of the VPN path on Linux (run in Docker, see
// docs in build-android.ps1): a real TUN fd goes into StartTun, and TCP +
// UDP through it must reach the internet via a phone helper at $PHONE.
package droid

import (
	"bytes"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/hashicorp/yamux"
	"golang.org/x/sys/unix"

	"teltun/internal/wire"
)

type nopListener struct{ t *testing.T }

func (l nopListener) OnStatus(s, step, e, d string)           { l.t.Log("status", s, step, e, d) }
func (l nopListener) OnStats(_, _ float64, _, _ int64, _ int) {}
func (l nopListener) OnLog(line string)                       { l.t.Log(line) }
func (l nopListener) OnAuthPrompt()                           {}

func openTun(t *testing.T, name string) int {
	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	var ifr [unix.IFNAMSIZ + 64]byte
	copy(ifr[:], name)
	*(*uint16)(unsafe.Pointer(&ifr[unix.IFNAMSIZ])) = unix.IFF_TUN | unix.IFF_NO_PI
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TUNSETIFF, uintptr(unsafe.Pointer(&ifr[0]))); errno != 0 {
		t.Fatal(errno)
	}
	return fd
}

func sh(t *testing.T, cmd string) string {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v %s", cmd, err, out)
	}
	return string(out)
}

func TestTunThroughPhone(t *testing.T) {
	phone := os.Getenv("PHONE") // host:1081 of a container running the helper
	c, err := net.Dial("tcp", phone)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := yamux.Client(c, wire.Config())
	if err != nil {
		t.Fatal(err)
	}
	e := New(t.TempDir(), nopListener{t})
	e.sess = sess

	fd := openTun(t, "tt0")
	sh(t, "ip addr add 172.19.0.1/30 dev tt0 && ip link set tt0 up")
	// Only these go through the tunnel; the link to the helper stays outside.
	sh(t, "ip route add 1.1.1.1/32 dev tt0 && ip route add 1.0.0.1/32 dev tt0")
	if err := e.StartTun(fd, true); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()

	// TCP: HTTPS to 1.1.1.1 through the TUN.
	hc := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{ServerName: "one.one.one.one"}}}
	r, err := hc.Get("https://1.1.1.1/cdn-cgi/trace")
	if err != nil {
		t.Fatal("tcp:", err)
	}
	var b bytes.Buffer
	b.ReadFrom(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || !strings.Contains(b.String(), "ip=") {
		t.Fatalf("tcp: http %d %q", r.StatusCode, b.String())
	}
	for _, ln := range strings.Split(b.String(), "\n") {
		if strings.HasPrefix(ln, "ip=") {
			t.Log("tcp ok, public", ln)
		}
	}

	// UDP: a DNS query to 1.0.0.1:53 through the TUN.
	uc, err := net.Dial("udp", "1.0.0.1:53")
	if err != nil {
		t.Fatal(err)
	}
	q := []byte{0xab, 0xcd, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0, 0, 1, 0, 1}
	uc.Write(q)
	uc.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp := make([]byte, 512)
	n, err := uc.Read(resp)
	if err != nil || n < 12 || resp[0] != 0xab {
		t.Fatalf("udp: %v n=%d", err, n)
	}
	t.Logf("udp ok, %d byte DNS answer", n)

	// QUIC to 443 must be refused when blockQUIC is on.
	qc, _ := net.Dial("udp", "1.1.1.1:443")
	qc.Write([]byte("quic"))
	qc.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := qc.Read(resp); err == nil {
		t.Fatal("quic was not blocked")
	}
	t.Logf("counters: down=%d up=%d", e.down.Load(), e.up.Load())
}
