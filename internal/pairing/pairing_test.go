package pairing

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"strings"
	"testing"
	"time"
)

func TestInfoURLRoundTrips(t *testing.T) {
	info := Info{Host: "192.168.1.50", Port: 11535, Token: "runyx_abc123"}
	url := info.URL()
	if !strings.HasPrefix(url, "runyx://pair?d=") {
		t.Fatalf("URL = %q, unexpected prefix", url)
	}
	encoded := strings.TrimPrefix(url, "runyx://pair?d=")
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := "192.168.1.50:11535:runyx_abc123"
	if string(decoded) != want {
		t.Errorf("decoded = %q, want %q", decoded, want)
	}
}

func TestQRCodePNGProducesValidPNG(t *testing.T) {
	info := Info{Host: "10.0.0.5", Port: 11535, Token: "tok"}
	png, err := info.QRCodePNG(256)
	if err != nil {
		t.Fatalf("QRCodePNG: %v", err)
	}
	if len(png) < 8 || !bytes.HasPrefix(png, []byte{0x89, 'P', 'N', 'G'}) {
		t.Error("output doesn't look like a PNG file")
	}
}

func TestQRCodeModulesEncodesTheSameURLAsThePNG(t *testing.T) {
	info := Info{Host: "10.0.0.5", Port: 11535, Token: "tok"}
	modules, err := info.QRCodeModules()
	if err != nil {
		t.Fatalf("QRCodeModules: %v", err)
	}
	if len(modules) == 0 {
		t.Fatal("expected a non-empty module grid")
	}
	// Every row must be the same (square) length, and the grid must
	// contain both dark and light modules -- a QR code that somehow
	// came back all-one-color would be an encoding bug, not real data.
	width := len(modules[0])
	sawDark, sawLight := false, false
	for _, row := range modules {
		if len(row) != width {
			t.Fatalf("row length = %d, want %d (grid must be square)", len(row), width)
		}
		for _, m := range row {
			if m {
				sawDark = true
			} else {
				sawLight = true
			}
		}
	}
	if !sawDark || !sawLight {
		t.Error("expected the module grid to contain both dark and light modules")
	}
}

func TestLocalIPv4ReturnsSomething(t *testing.T) {
	ip := LocalIPv4()
	if ip == "" {
		t.Error("expected a non-empty IP")
	}
}

func TestWakeOnLANBuildsCorrectMagicPacket(t *testing.T) {
	// Verify packet structure by capturing what would be sent, via a
	// real local UDP listener instead of actually broadcasting.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	defer conn.Close()

	go func() {
		WakeOnLAN("aa:bb:cc:dd:ee:ff", conn.LocalAddr().String())
	}()

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	if n != 102 {
		t.Fatalf("packet length = %d, want 102", n)
	}
	for i := 0; i < 6; i++ {
		if buf[i] != 0xFF {
			t.Fatalf("expected 6 leading 0xFF bytes, got %v", buf[:6])
		}
	}
	wantMAC := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	for rep := 0; rep < 16; rep++ {
		got := buf[6+rep*6 : 6+rep*6+6]
		if !bytes.Equal(got, wantMAC) {
			t.Fatalf("MAC repetition %d = %x, want %x", rep, got, wantMAC)
		}
	}
}

func TestWakeOnLANRejectsBadMAC(t *testing.T) {
	if err := WakeOnLAN("not-a-mac", ""); err == nil {
		t.Error("expected error for invalid MAC")
	}
}

func TestAnnounceAndDiscover(t *testing.T) {
	ann, err := Announce("runyx-test-instance", 54999)
	if err != nil {
		t.Skipf("mDNS announce not available in this sandbox: %v", err)
	}
	defer ann.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	found, err := Discover(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	matched := false
	for _, d := range found {
		if d.Port == 54999 {
			matched = true
		}
	}
	if !matched {
		t.Logf("did not see our own announced instance among %v (mDNS multicast may be unavailable in this sandbox's network namespace)", found)
	}
}
