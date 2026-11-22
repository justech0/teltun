package adbusb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

// Push copies data to path on the phone with the given mode (e.g. 0755),
// using the adb "sync:" service.
func (c *Conn) Push(data []byte, path string, mode uint32) error {
	s, err := c.Open("sync:")
	if err != nil {
		return err
	}
	defer s.Close()

	req := func(id string, n int) []byte {
		b := make([]byte, 8)
		copy(b, id)
		binary.LittleEndian.PutUint32(b[4:], uint32(n))
		return b
	}
	spec := fmt.Sprintf("%s,%d", path, mode)
	if _, err := s.Write(append(req("SEND", len(spec)), spec...)); err != nil {
		return err
	}
	for off := 0; off < len(data); off += 64 * 1024 {
		end := min(off+64*1024, len(data))
		if _, err := s.Write(append(req("DATA", end-off), data[off:end]...)); err != nil {
			return err
		}
	}
	if _, err := s.Write(req("DONE", int(time.Now().Unix()))); err != nil {
		return err
	}
	resp := make([]byte, 8)
	if _, err := io.ReadFull(s, resp); err != nil {
		return err
	}
	if string(resp[:4]) == "OKAY" {
		return nil
	}
	msg := make([]byte, binary.LittleEndian.Uint32(resp[4:]))
	io.ReadFull(s, msg)
	return errors.New("adb push: " + string(msg))
}
