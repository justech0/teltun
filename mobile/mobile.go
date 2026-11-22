// Package mobile is the API the Android / ChromeOS app calls, made into a
// Java library with gomobile. It only forwards to internal/droid, using
// types gomobile can bind (int, float64, string, []byte, interfaces).
package mobile

import (
	"sync"

	"teltun/internal/droid"
	"teltun/internal/wire"
)

// USB is implemented in Kotlin with UsbDeviceConnection.bulkTransfer.
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

var (
	mu  sync.Mutex
	eng *droid.Engine
)

// Init must be called once, with a private folder for the adb key.
func Init(dataDir string, l Listener) {
	mu.Lock()
	defer mu.Unlock()
	if eng == nil {
		eng = droid.New(dataDir, l)
	}
}

func engine() *droid.Engine {
	mu.Lock()
	defer mu.Unlock()
	return eng
}

// Connect opens the USB link to the phone. It blocks; call it off the UI thread.
func Connect(usb USB) error { return engine().Connect(usb) }

// StartTun routes the VPN descriptor through the phone.
func StartTun(fd int, blockQUIC bool) error { return engine().StartTun(fd, blockQUIC) }

// Stop tears the tunnel down.
func Stop() {
	if e := engine(); e != nil {
		e.Stop()
	}
}

// ProtocolVersion is the PC <-> phone protocol version.
func ProtocolVersion() string { return wire.Version }
