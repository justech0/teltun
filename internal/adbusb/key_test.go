package adbusb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The encoding must match what adb itself writes to ~/.android/adbkey.pub.
func TestPublicKeyMatchesAdb(t *testing.T) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".android")
	want, err := os.ReadFile(filepath.Join(dir, "adbkey.pub"))
	if err != nil {
		t.Skip("no ~/.android/adbkey.pub")
	}
	k, err := LoadKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := PublicKey(&k.PublicKey)
	if exp := strings.Fields(string(want))[0]; got != exp {
		t.Fatalf("public key mismatch\n got %s...\nwant %s...", got[:40], exp[:40])
	}
}
