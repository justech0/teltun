// Package adbusb speaks the adb protocol straight to a phone's USB
// endpoints, with no adb program in between. The Android / ChromeOS app
// uses it: Android hands us the USB interface, we do the rest.
//
// Protocol: every message is a 24-byte header (command, arg0, arg1, payload
// length, payload checksum, command^0xffffffff) followed by the payload.
package adbusb

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	cmdCNXN = 0x4e584e43
	cmdAUTH = 0x48545541
	cmdOPEN = 0x4e45504f
	cmdOKAY = 0x59414b4f
	cmdCLSE = 0x45534c43
	cmdWRTE = 0x45545257

	version    = 0x01000001
	maxPayload = 256 * 1024

	authToken     = 1
	authSignature = 2
	authPublicKey = 3
)

// Transport moves bytes over the USB bulk endpoints. Each Write is one bulk
// OUT transfer; each Read is one bulk IN transfer of up to len(p) bytes.
type Transport interface {
	Write(p []byte) (int, error)
	Read(p []byte) (int, error)
}

type message struct {
	cmd, arg0, arg1 uint32
	data            []byte
}

// Conn is one adb session with a phone.
type Conn struct {
	tr         Transport
	maxPayload int
	Banner     string // "device::ro.product.model=...;..."

	wmu     sync.Mutex
	mu      sync.Mutex
	streams map[uint32]*Stream
	nextID  uint32
	closed  chan struct{}
	err     error
}

func (c *Conn) send(m message) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	var sum uint32
	for _, b := range m.data {
		sum += uint32(b)
	}
	h := make([]byte, 24)
	binary.LittleEndian.PutUint32(h[0:], m.cmd)
	binary.LittleEndian.PutUint32(h[4:], m.arg0)
	binary.LittleEndian.PutUint32(h[8:], m.arg1)
	binary.LittleEndian.PutUint32(h[12:], uint32(len(m.data)))
	binary.LittleEndian.PutUint32(h[16:], sum)
	binary.LittleEndian.PutUint32(h[20:], m.cmd^0xffffffff)
	if _, err := c.tr.Write(h); err != nil {
		return err
	}
	if len(m.data) > 0 {
		if _, err := c.tr.Write(m.data); err != nil {
			return err
		}
	}
	return nil
}

func readFull(tr Transport, p []byte) error {
	for n := 0; n < len(p); {
		k, err := tr.Read(p[n:])
		if err != nil {
			return err
		}
		n += k
	}
	return nil
}

func (c *Conn) recv() (message, error) {
	h := make([]byte, 24)
	if err := readFull(c.tr, h); err != nil {
		return message{}, err
	}
	m := message{
		cmd:  binary.LittleEndian.Uint32(h[0:]),
		arg0: binary.LittleEndian.Uint32(h[4:]),
		arg1: binary.LittleEndian.Uint32(h[8:]),
	}
	if binary.LittleEndian.Uint32(h[20:]) != m.cmd^0xffffffff {
		return m, errors.New("adb: bad message header")
	}
	if n := binary.LittleEndian.Uint32(h[12:]); n > 0 {
		if n > 1<<20 {
			return m, fmt.Errorf("adb: payload too big (%d)", n)
		}
		m.data = make([]byte, n)
		if err := readFull(c.tr, m.data); err != nil {
			return m, err
		}
	}
	return m, nil
}

// Connect does the CNXN/AUTH handshake. If the phone does not know our key
// yet, onPrompt is called and we wait (up to wait) for the user to tap
// "Allow" on the phone.
func Connect(tr Transport, key *rsa.PrivateKey, name string, onPrompt func(), wait time.Duration) (*Conn, error) {
	c := &Conn{tr: tr, maxPayload: maxPayload, streams: map[uint32]*Stream{}, nextID: 1, closed: make(chan struct{})}
	if err := c.send(message{cmdCNXN, version, maxPayload, []byte("host::\x00")}); err != nil {
		return nil, err
	}
	sentSig, sentKey := false, false
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return nil, errors.New("adb: no answer from the phone")
		}
		m, err := c.recv()
		if err != nil {
			return nil, err
		}
		switch m.cmd {
		case cmdCNXN:
			if int(m.arg1) > 0 && int(m.arg1) < c.maxPayload {
				c.maxPayload = int(m.arg1)
			}
			c.Banner = strings.TrimRight(string(m.data), "\x00")
			go c.loop()
			return c, nil
		case cmdAUTH:
			if m.arg0 != authToken {
				continue
			}
			switch {
			case !sentSig:
				// The token is signed as if it were a SHA-1 digest.
				sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, m.data)
				if err != nil {
					return nil, err
				}
				sentSig = true
				if err := c.send(message{cmdAUTH, authSignature, 0, sig}); err != nil {
					return nil, err
				}
			case !sentKey:
				pub := PublicKey(&key.PublicKey) + " " + name + "\x00"
				sentKey = true
				if onPrompt != nil {
					onPrompt()
				}
				deadline = time.Now().Add(wait)
				if err := c.send(message{cmdAUTH, authPublicKey, 0, []byte(pub)}); err != nil {
					return nil, err
				}
			}
		}
	}
}

// Prop reads a property from the CNXN banner, e.g. "ro.product.model".
func (c *Conn) Prop(name string) string {
	_, rest, _ := strings.Cut(c.Banner, "::")
	for _, kv := range strings.Split(rest, ";") {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v
		}
	}
	return ""
}

func (c *Conn) loop() {
	var err error
	for {
		var m message
		m, err = c.recv()
		if err != nil {
			break
		}
		c.mu.Lock()
		s := c.streams[m.arg1]
		c.mu.Unlock()
		if s == nil {
			continue
		}
		switch m.cmd {
		case cmdOKAY:
			s.okay(m.arg0)
		case cmdWRTE:
			s.push(m.data)
			c.send(message{cmdOKAY, s.local, s.remote, nil})
		case cmdCLSE:
			s.remoteClosed()
		}
	}
	c.mu.Lock()
	c.err = err
	for _, s := range c.streams {
		s.remoteClosed()
	}
	c.mu.Unlock()
	close(c.closed)
}

// Done is closed when the USB link breaks.
func (c *Conn) Done() <-chan struct{} { return c.closed }

// Open starts a service on the phone: "shell:cmd", "sync:", "tcp:1081"...
func (c *Conn) Open(service string) (*Stream, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	s := newStream(c, id)
	c.streams[id] = s
	c.mu.Unlock()
	if err := c.send(message{cmdOPEN, id, 0, []byte(service + "\x00")}); err != nil {
		return nil, err
	}
	select {
	case <-s.opened:
		return s, nil
	case <-s.done:
		return nil, fmt.Errorf("adb: phone refused %q", service)
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("adb: no answer opening %q", service)
	}
}

// Shell runs a command and returns its output.
func (c *Conn) Shell(cmd string) (string, error) {
	s, err := c.Open("shell:" + cmd)
	if err != nil {
		return "", err
	}
	defer s.Close()
	out, err := io.ReadAll(s)
	return string(out), err
}
