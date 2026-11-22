package engine

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

// RunHelper is the privileged side of startTun on Linux and macOS: it
// creates the TelTun card and keeps it until the app drops the control link.
// main() calls it when the binary is started with --tun-helper.
func RunHelper(args []string) int {
	fs := flag.NewFlagSet("tun-helper", flag.ContinueOnError)
	ctl := fs.String("ctl", "", "control address of the app")
	token := fs.String("token", "", "shared secret")
	socks := fs.Int("socks", 0, "local SOCKS port of the USB link")
	blockQUIC := fs.Bool("block-quic", false, "reject UDP 443")
	logDir := fs.String("log", "", "folder for sing-box.log")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *logDir != "" {
		boxLogDir = *logDir
	}

	conn, err := net.Dial("tcp", *ctl)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tun-helper:", err)
		return 1
	}
	defer conn.Close()
	fmt.Fprintln(conn, *token)

	b, err := startBox(*socks, *blockQUIC)
	if err != nil {
		fmt.Fprintf(conn, "error: %v\n", err)
		return 1
	}
	defer b.Close()
	fmt.Fprintln(conn, "ready")

	// Stay up until the app closes the link (disconnect, quit or crash).
	gone := make(chan struct{})
	go func() {
		bufio.NewReader(conn).ReadString('\n')
		close(gone)
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-gone:
	case <-sig:
	}
	return 0
}
