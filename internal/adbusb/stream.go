package adbusb

import (
	"io"
	"sync"
)

// Stream is one open adb service. It is a plain io.ReadWriteCloser, so the
// USB link to the phone helper can carry yamux exactly like on the desktop.
type Stream struct {
	c      *Conn
	local  uint32
	remote uint32

	opened   chan struct{}
	openOnce sync.Once
	ack      chan struct{} // one WRTE in flight until the phone says OKAY
	done     chan struct{}
	doneOnce sync.Once

	mu  sync.Mutex
	buf [][]byte
	in  chan struct{}
}

func newStream(c *Conn, id uint32) *Stream {
	return &Stream{c: c, local: id, opened: make(chan struct{}), ack: make(chan struct{}, 1),
		done: make(chan struct{}), in: make(chan struct{}, 1)}
}

func (s *Stream) okay(remote uint32) {
	first := false
	s.openOnce.Do(func() {
		s.remote = remote
		close(s.opened)
		first = true
	})
	if !first {
		select {
		case s.ack <- struct{}{}:
		default:
		}
	}
}

func (s *Stream) push(p []byte) {
	s.mu.Lock()
	s.buf = append(s.buf, p)
	s.mu.Unlock()
	select {
	case s.in <- struct{}{}:
	default:
	}
}

func (s *Stream) remoteClosed() {
	s.doneOnce.Do(func() { close(s.done) })
}

func (s *Stream) Read(p []byte) (int, error) {
	for {
		s.mu.Lock()
		if len(s.buf) > 0 {
			n := copy(p, s.buf[0])
			if n == len(s.buf[0]) {
				s.buf = s.buf[1:]
			} else {
				s.buf[0] = s.buf[0][n:]
			}
			s.mu.Unlock()
			return n, nil
		}
		s.mu.Unlock()
		select {
		case <-s.in:
		case <-s.done:
			s.mu.Lock()
			empty := len(s.buf) == 0
			s.mu.Unlock()
			if empty {
				return 0, io.EOF
			}
		}
	}
}

func (s *Stream) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		n := len(p)
		if n > s.c.maxPayload {
			n = s.c.maxPayload
		}
		chunk := make([]byte, n)
		copy(chunk, p[:n])
		if err := s.c.send(message{cmdWRTE, s.local, s.remote, chunk}); err != nil {
			return total, err
		}
		select {
		case <-s.ack:
		case <-s.done:
			return total, io.ErrClosedPipe
		}
		total += n
		p = p[n:]
	}
	return total, nil
}

func (s *Stream) Close() error {
	select {
	case <-s.done:
	default:
		s.c.send(message{cmdCLSE, s.local, s.remote, nil})
		s.remoteClosed()
	}
	s.c.mu.Lock()
	delete(s.c.streams, s.local)
	s.c.mu.Unlock()
	return nil
}
