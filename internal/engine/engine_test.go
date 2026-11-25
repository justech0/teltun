package engine

import "testing"

func TestPickDevice(t *testing.T) {
	samsung := Device{Serial: "R58", State: "device", Model: "SM A165F"}
	redmi := Device{Serial: "fec04109", State: "device"}
	locked := Device{Serial: "X1", State: "unauthorized"}

	cases := []struct {
		name      string
		devs      []Device
		preferred string
		want      string
	}{
		{"none", nil, "", ""},
		{"no preference", []Device{samsung}, "", "R58"},
		{"preferred not plugged in", []Device{samsung}, "fec04109", "R58"},
		{"preferred plugged in", []Device{samsung, redmi}, "fec04109", "fec04109"},
		{"ready before locked", []Device{locked, samsung}, "", "R58"},
		{"only locked, report it", []Device{locked}, "", "X1"},
	}
	for _, c := range cases {
		got := ""
		if d := pickDevice(c.devs, c.preferred); d != nil {
			got = d.Serial
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
