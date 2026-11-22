// Package hotspot shares the PC's connection (the TelTun card) over Wi-Fi:
// Windows "Mobile hotspot" on Windows, a NetworkManager hotspot on Linux.
package hotspot

type Info struct {
	OK      bool   `json:"ok"`
	State   string `json:"state"` // On, Off, InTransition, Unknown, Unsupported
	SSID    string `json:"ssid"`
	Pass    string `json:"pass"`
	Clients int    `json:"clients"`
	Max     int    `json:"max"`
	Profile string `json:"profile"`
	Msg     string `json:"msg"`
}
