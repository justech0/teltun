package hotspot

// Linux: NetworkManager's hotspot mode (nmcli). It shares the default route,
// which is the TelTun card while the tunnel is up.

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const conName = "TelTun-Hotspot"

func nmcli(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nmcli", args...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			s = err.Error()
		}
		return s, errors.New(s)
	}
	return s, nil
}

func available() error {
	if _, err := exec.LookPath("nmcli"); err != nil {
		return errors.New("NetworkManager (nmcli) not found")
	}
	return nil
}

// wifiDevice returns the first Wi-Fi adapter NetworkManager manages.
func wifiDevice() (string, error) {
	out, err := nmcli("-t", "-f", "DEVICE,TYPE", "device")
	if err != nil {
		return "", err
	}
	for _, ln := range strings.Split(out, "\n") {
		if dev, typ, ok := strings.Cut(ln, ":"); ok && typ == "wifi" {
			return dev, nil
		}
	}
	return "", errors.New("no Wi-Fi adapter found")
}

func Status() (Info, error) {
	if err := available(); err != nil {
		return Info{State: "Unsupported", Msg: err.Error()}, nil
	}
	info := Info{OK: true, State: "Off", Profile: conName}
	active, err := nmcli("-t", "-f", "NAME", "connection", "show", "--active")
	if err != nil {
		return info, err
	}
	for _, n := range strings.Split(active, "\n") {
		if n == conName {
			info.State = "On"
		}
	}
	if ssid, err := nmcli("-t", "-g", "802-11-wireless.ssid", "connection", "show", conName); err == nil {
		info.SSID = ssid
		info.Pass, _ = nmcli("-s", "-t", "-g", "802-11-wireless-security.psk", "connection", "show", conName)
	}
	return info, nil
}

// Start turns the hotspot on. Empty ssid/pass keep the last values (or use
// "TelTun" and a password NetworkManager makes up the first time).
func Start(ssid, pass string) (Info, error) {
	if err := available(); err != nil {
		return Info{State: "Unsupported", Msg: err.Error()}, err
	}
	dev, err := wifiDevice()
	if err != nil {
		return Info{State: "Unknown", Msg: err.Error()}, err
	}
	if cur, err := Status(); err == nil && cur.State == "On" &&
		(ssid == "" || ssid == cur.SSID) && (pass == "" || pass == cur.Pass) {
		return cur, nil
	}
	if ssid == "" {
		if cur, _ := Status(); cur.SSID != "" {
			ssid = cur.SSID
		} else {
			ssid = "TelTun"
		}
	}
	args := []string{"device", "wifi", "hotspot", "ifname", dev, "con-name", conName, "ssid", ssid}
	if pass != "" {
		args = append(args, "password", pass)
	}
	if _, err := nmcli(args...); err != nil {
		return Info{State: "Unknown", Msg: err.Error()}, err
	}
	return Status()
}

func Stop() (Info, error) {
	if err := available(); err != nil {
		return Info{State: "Unsupported", Msg: err.Error()}, err
	}
	if cur, err := Status(); err == nil && cur.State != "On" {
		return cur, nil
	}
	if _, err := nmcli("connection", "down", conName); err != nil {
		return Info{State: "Unknown", Msg: err.Error()}, err
	}
	return Status()
}
