package main

import "encoding/binary"

// trayIcon returns the largest image inside the .ico. Our icons store PNG
// images, which is what the Linux tray (StatusNotifierItem) can show.
func trayIcon(ico []byte) []byte {
	if len(ico) < 6 {
		return ico
	}
	n := int(binary.LittleEndian.Uint16(ico[4:]))
	best, bestW := []byte(nil), -1
	for i := 0; i < n; i++ {
		e := 6 + 16*i
		if e+16 > len(ico) {
			break
		}
		w := int(ico[e])
		if w == 0 {
			w = 256
		}
		size := int(binary.LittleEndian.Uint32(ico[e+8:]))
		off := int(binary.LittleEndian.Uint32(ico[e+12:]))
		if off+size <= len(ico) && w > bestW {
			best, bestW = ico[off:off+size], w
		}
	}
	if best == nil {
		return ico
	}
	return best
}
