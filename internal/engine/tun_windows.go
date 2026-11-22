package engine

import "io"

// startTun creates the TelTun card inside this process. The Windows build
// already runs as admin (see the exe manifest).
func startTun(socksPort int, blockQUIC bool) (io.Closer, error) {
	return startBox(socksPort, blockQUIC)
}
