// adbtest drives a phone or emulator with TelTun's own adb client (no adb
// program) over TCP, the same wire protocol the Android app uses over USB:
// handshake, push the helper, start it, open the link and go online.
//
//	go run ./cmd/adbtest -addr 127.0.0.1:5601 -bin assets/phone/teltun-amd64
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/yamux"
	"golang.org/x/net/proxy"

	"teltun/internal/adbusb"
	"teltun/internal/wire"
)

type tcpTransport struct{ net.Conn }

type linkDialer struct{ s *yamux.Session }

func (d linkDialer) Dial(network, addr string) (net.Conn, error) { return d.s.Open() }

func main() {
	addr := flag.String("addr", "127.0.0.1:5601", "adbd TCP address (emulator: console port + 1)")
	bin := flag.String("bin", "assets/phone/teltun-amd64", "phone helper binary")
	keyDir := flag.String("key", "", "folder with adbkey (default ~/.android)")
	apk := flag.String("apk", "", "install this APK, start TelTun and exit")
	shot := flag.String("shot", "", "save a screenshot (PNG) to this file and exit")
	shCmd := flag.String("sh", "", "run one shell command, print its output and exit")
	flag.Parse()
	if *keyDir == "" {
		home, _ := os.UserHomeDir()
		*keyDir = filepath.Join(home, ".android")
	}

	c, err := net.Dial("tcp", *addr)
	must(err)
	key, err := adbusb.LoadKey(*keyDir)
	must(err)
	t0 := time.Now()
	conn, err := adbusb.Connect(tcpTransport{c}, key, "teltun@adbtest",
		func() { fmt.Println("-> allow the key on the device") }, time.Minute)
	must(err)
	fmt.Printf("connected in %v: model=%q\n", time.Since(t0).Round(time.Millisecond), conn.Prop("ro.product.model"))

	if *apk != "" {
		b, err := os.ReadFile(*apk)
		must(err)
		must(conn.Push(b, "/data/local/tmp/teltun.apk", 0o644))
		out, _ := conn.Shell("pm install -r /data/local/tmp/teltun.apk && am start -n app.teltun/.MainActivity")
		fmt.Println(strings.TrimSpace(out))
		return
	}
	if *shCmd != "" {
		out, err := conn.Shell(*shCmd)
		must(err)
		fmt.Print(out)
		return
	}
	if *shot != "" {
		s, err := conn.Open("exec:screencap -p")
		must(err)
		png, err := io.ReadAll(s)
		must(err)
		must(os.WriteFile(*shot, png, 0o644))
		fmt.Println("screenshot", len(png), "bytes")
		return
	}

	out, err := conn.Shell("uname -m; getprop ro.build.version.release")
	must(err)
	fmt.Println("shell:", strings.Join(strings.Fields(out), " "))

	data, err := os.ReadFile(*bin)
	must(err)
	t0 = time.Now()
	must(conn.Push(data, "/data/local/tmp/teltun", 0o755))
	fmt.Printf("push %.1f MB in %v\n", float64(len(data))/1e6, time.Since(t0).Round(time.Millisecond))

	helper, err := conn.Open("shell:exec /data/local/tmp/teltun")
	must(err)
	line, _ := bufio.NewReader(helper).ReadString('\n')
	fmt.Println("helper:", strings.TrimSpace(line))

	link, err := conn.Open(fmt.Sprintf("tcp:%d", wire.PhonePort))
	must(err)
	sess, err := yamux.Client(link, wire.Config())
	must(err)
	rtt, err := sess.Ping()
	must(err)
	fmt.Println("yamux ping:", rtt)

	d, _ := proxy.SOCKS5("tcp", "phone", nil, linkDialer{sess})
	hc := &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{DialContext: d.(proxy.ContextDialer).DialContext}}
	r, err := hc.Get("https://api.ipify.org")
	must(err)
	ip, _ := io.ReadAll(r.Body)
	r.Body.Close()
	fmt.Println("public ip via device:", string(ip))

	t0 = time.Now()
	r, err = hc.Get("https://speed.cloudflare.com/__down?bytes=20000000")
	must(err)
	n, _ := io.Copy(io.Discard, r.Body)
	r.Body.Close()
	sec := time.Since(t0).Seconds()
	fmt.Printf("download %.1f MB in %.1fs = %.0f Mbps\n", float64(n)/1e6, sec, float64(n)*8/1e6/sec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	<-ctx.Done()
	helper.Close()
	conn.Shell("pkill -f /data/local/tmp/teltun")
}

func must(err error) {
	if err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}
}
