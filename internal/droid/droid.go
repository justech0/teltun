// Package droid is the TelTun engine for the Android / ChromeOS app.
//
// The desktop app drives adb and sing-box. Here Android gives us two raw
// things instead: the phone's USB interface (bulk endpoints) and a VPN file
// descriptor. We speak adb ourselves over USB, start the same phone helper,
// open the same single yamux link, and feed every VPN connection into it.
package droid

import (
	"bufio"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/logger"
	"golang.org/x/net/proxy"

	"teltun/internal/adbusb"
	"teltun/internal/wire"
)

//go:embed phone
var phoneFS embed.FS

const phonePath = "/data/local/tmp/teltun"

// USB is implemented by the app with UsbDeviceConnection.bulkTransfer.
type USB interface {
	BulkOut(data []byte) (int, error)
	BulkIn(max int) ([]byte, error)
}

// Listener receives progress for the UI. err is an i18n key.
type Listener interface {
	OnStatus(state, step, err, detail string)
	OnStats(down, up float64, totalDown, totalUp int64, streams int)
	OnLog(line string)
	OnAuthPrompt()
}

type usbTransport struct{ u USB }

func (t usbTransport) Write(p []byte) (int, error) { return t.u.BulkOut(p) }
func (t usbTransport) Read(p []byte) (int, error) {
	b, err := t.u.BulkIn(len(p))
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, errors.New("usb: empty read")
	}
	return copy(p, b), nil
}

type Engine struct {
	dataDir string
	l       Listener

	mu     sync.Mutex
	conn   *adbusb.Conn
	helper *adbusb.Stream
	sess   *yamux.Session
	stack  tun.Stack
	tunDev tun.Tun
	stop   chan struct{}
	device string

	down, up atomic.Uint64
	streams  atomic.Int64
}

func New(dataDir string, l Listener) *Engine { return &Engine{dataDir: dataDir, l: l} }

func (e *Engine) logf(format string, a ...any) {
	e.l.OnLog(time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, a...))
}

func (e *Engine) status(state, step, errKey, detail string) {
	if step != "" {
		e.logf("» %s", step)
	}
	e.l.OnStatus(state, step, errKey, detail)
}

type stepErr struct {
	key string
	err error
}

func (s *stepErr) Error() string { return s.key + ": " + s.err.Error() }

func fail(key string, err error) error { return &stepErr{key, err} }

// Connect opens the USB link to the phone (everything except the VPN).
// It blocks, so call it off the UI thread.
func (e *Engine) Connect(u USB) error {
	err := e.connect(u)
	if err != nil {
		e.logf("error: %v", err)
		e.Stop()
		key, detail := "errUnknown", err.Error()
		var se *stepErr
		if errors.As(err, &se) {
			key, detail = se.key, se.err.Error()
		}
		e.status("idle", "", key, detail)
	}
	return err
}

func (e *Engine) connect(u USB) error {
	e.status("connecting", "stepDevice", "", "")
	key, err := adbusb.LoadKey(e.dataDir)
	if err != nil {
		return fail("errPrepare", err)
	}
	conn, err := adbusb.Connect(usbTransport{u}, key, "teltun@chromebook", e.l.OnAuthPrompt, 2*time.Minute)
	if err != nil {
		return fail("errUnauthorized", err)
	}
	e.mu.Lock()
	e.conn = conn
	e.device = conn.Prop("ro.product.model")
	e.mu.Unlock()
	e.logf("device: %s", e.device)

	e.status("connecting", "stepPush", "", "")
	arch, _ := conn.Shell("uname -m")
	arch = strings.TrimSpace(arch)
	bin := "teltun-arm64"
	switch {
	case strings.Contains(arch, "aarch64"), strings.Contains(arch, "arm64"):
	case strings.HasPrefix(arch, "arm"):
		bin = "teltun-arm"
	case arch == "x86_64":
		bin = "teltun-amd64"
	}
	data, err := phoneFS.ReadFile("phone/" + bin)
	if err != nil {
		return fail("errPrepare", fmt.Errorf("helper %s missing from the app", bin))
	}
	e.logf("phone cpu: %s -> %s", arch, bin)
	conn.Shell("pkill -f " + phonePath)
	if err := conn.Push(data, phonePath, 0o755); err != nil {
		return fail("errPush", err)
	}

	e.status("connecting", "stepPhone", "", "")
	helper, err := conn.Open("shell:exec " + phonePath)
	if err != nil {
		return fail("errPhone", err)
	}
	e.mu.Lock()
	e.helper = helper
	e.mu.Unlock()
	ready := make(chan struct{}, 1)
	go func() {
		sc := bufio.NewScanner(helper)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			e.logf("phone: %s", line)
			if strings.Contains(line, "ready") {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		return fail("errPhone", errors.New("phone helper did not start"))
	}

	e.status("connecting", "stepLink", "", "")
	link, err := conn.Open(fmt.Sprintf("tcp:%d", wire.PhonePort))
	if err != nil {
		return fail("errLink", err)
	}
	sess, err := yamux.Client(link, wire.Config())
	if err != nil {
		return fail("errLink", err)
	}
	if _, err := sess.Ping(); err != nil {
		return fail("errLink", err)
	}
	e.mu.Lock()
	e.sess = sess
	e.mu.Unlock()

	e.status("connecting", "stepTest", "", "")
	if err := e.selfTest(); err != nil {
		return fail("errTest", err)
	}
	e.logf("internet via phone OK")
	e.status("linked", "", "", "")
	return nil
}

func (e *Engine) selfTest() error {
	d, _ := proxy.SOCKS5("tcp", "phone", nil, linkDialer{e})
	hc := &http.Client{Timeout: 15 * time.Second,
		Transport: &http.Transport{DialContext: d.(proxy.ContextDialer).DialContext}}
	for _, u := range []string{"http://connectivitycheck.gstatic.com/generate_204", "http://cp.cloudflare.com/generate_204"} {
		r, err := hc.Get(u)
		if err != nil {
			continue
		}
		r.Body.Close()
		if r.StatusCode == 204 || r.StatusCode == 200 {
			return nil
		}
	}
	return errors.New("no internet through the phone")
}

type linkDialer struct{ e *Engine }

func (d linkDialer) Dial(_, _ string) (net.Conn, error) { return d.e.openStream() }

func (e *Engine) openStream() (net.Conn, error) {
	e.mu.Lock()
	s := e.sess
	e.mu.Unlock()
	if s == nil {
		return nil, errors.New("not connected")
	}
	return s.Open()
}

// StartTun takes the VPN descriptor from VpnService and routes everything
// in it through the phone.
func (e *Engine) StartTun(fd int, blockQUIC bool) error {
	opts := tun.Options{
		FileDescriptor: fd,
		MTU:            1500,
		Inet4Address:   []netip.Prefix{netip.MustParsePrefix("172.19.0.1/30")},
		Logger:         logger.NOP(),
	}
	dev, err := tun.New(opts)
	if err != nil {
		return fail("errTun", err)
	}
	st, err := tun.NewStack("gvisor", tun.StackOptions{
		Context:    context.Background(),
		Tun:        dev,
		TunOptions: opts,
		UDPTimeout: 5 * time.Minute,
		Handler:    &handler{e: e, blockQUIC: blockQUIC},
		Logger:     logger.NOP(),
	})
	if err != nil {
		dev.Close()
		return fail("errTun", err)
	}
	if err := st.Start(); err != nil {
		dev.Close()
		return fail("errTun", err)
	}
	e.mu.Lock()
	e.tunDev, e.stack = dev, st
	e.stop = make(chan struct{})
	stop, sess, conn := e.stop, e.sess, e.conn
	e.mu.Unlock()
	go e.watch(stop, sess, conn)
	e.logf("tunnel up")
	e.status("connected", "", "", e.device)
	return nil
}

func (e *Engine) watch(stop chan struct{}, sess *yamux.Session, conn *adbusb.Conn) {
	var usbDone <-chan struct{} // stays nil (never fires) without an adb link
	if conn != nil {
		usbDone = conn.Done()
	}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var lastD, lastU uint64
	last := time.Now()
	for {
		select {
		case <-stop:
			return
		case <-sess.CloseChan():
			e.lost()
			return
		case <-usbDone:
			e.lost()
			return
		case now := <-t.C:
			d, u := e.down.Load(), e.up.Load()
			sec := now.Sub(last).Seconds()
			e.l.OnStats(float64(d-lastD)/sec, float64(u-lastU)/sec, int64(d), int64(u), int(e.streams.Load()))
			lastD, lastU, last = d, u, now
		}
	}
}

func (e *Engine) lost() {
	e.logf("link lost (cable or phone)")
	e.Stop()
	e.status("idle", "", "lost", "")
}

// Stop tears everything down. The app closes the USB device afterwards.
func (e *Engine) Stop() {
	e.mu.Lock()
	st, dev, sess, helper, conn, stop := e.stack, e.tunDev, e.sess, e.helper, e.conn, e.stop
	e.stack, e.tunDev, e.sess, e.helper, e.conn, e.stop = nil, nil, nil, nil, nil, nil
	e.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	if st != nil {
		st.Close()
	}
	if dev != nil {
		dev.Close()
	}
	if sess != nil {
		sess.Close()
	}
	if helper != nil {
		helper.Close()
	}
	if conn != nil {
		done := make(chan struct{})
		go func() { conn.Shell("pkill -f " + phonePath); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
	e.down.Store(0)
	e.up.Store(0)
}

// counter counts bytes written through it.
type counter struct {
	w io.Writer
	n *atomic.Uint64
}

func (c counter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n.Add(uint64(n))
	return n, err
}
