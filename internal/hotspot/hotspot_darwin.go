package hotspot

import "errors"

// macOS has no public API to turn on Internet Sharing.
var errUnsupported = errors.New("unsupported")

func Status() (Info, error)                 { return Info{State: "Unsupported"}, nil }
func Stop() (Info, error)                   { return Info{State: "Unsupported"}, errUnsupported }
func Start(ssid, pass string) (Info, error) { return Info{State: "Unsupported"}, errUnsupported }
