// Package pairing implements Android device pairing (item 26): a QR
// code encoding a one-time pairing URL (device token + address), mDNS
// discovery so the phone can find the PC without typing an IP, and
// Wake-on-LAN to wake a sleeping PC. WireGuard/Tailscale-based remote
// access is documented but not implemented here — see
// docs/DECISIONS.md: it's a "bring your own VPN" opt-in, not something
// Runyx bundles.
package pairing

import (
	"encoding/base64"
	"fmt"
	"net"

	qrcode "github.com/skip2/go-qrcode"
)

// Info is what a pairing QR code encodes: enough for the phone to reach
// the PC and authenticate its first request.
type Info struct {
	Host  string `json:"host"` // LAN IP or hostname
	Port  int    `json:"port"`
	Token string `json:"token"` // one-time device token (see daemon.TokenStore)
}

// URL renders the pairing info as a runyx:// URL, which is what the QR
// code actually encodes (a generic JSON blob would also work, but a
// custom URL scheme lets a future Android app register as a handler
// and auto-import it from, say, a QR scanner app or a shared link).
func (i Info) URL() string {
	payload := fmt.Sprintf("%s:%d:%s", i.Host, i.Port, i.Token)
	return "runyx://pair?d=" + base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// QRCodePNG renders the pairing URL as a PNG QR code at the given pixel
// size (square).
func (i Info) QRCodePNG(size int) ([]byte, error) {
	return qrcode.Encode(i.URL(), qrcode.Medium, size)
}

// QRCodeModules returns the QR code's raw module grid (true = dark
// module) for a client that renders its own QR code instead of
// consuming a PNG -- the TUI, which has no image decoder and draws
// unicode blocks directly from a boolean grid the same way it does
// for the mascot's sprite grid.
func (i Info) QRCodeModules() ([][]bool, error) {
	qr, err := qrcode.New(i.URL(), qrcode.Medium)
	if err != nil {
		return nil, err
	}
	return qr.Bitmap(), nil
}

// IsLoopbackHost reports whether host (as found in a listen address, with
// no port) only binds the local machine: empty (Go's default "all
// interfaces" is NOT loopback, so that's handled separately by callers
// that care), "localhost", or an IP in a loopback range.
func IsLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// LocalIPv4 returns the machine's primary LAN IPv4 address (the one
// that would be used to reach the default route), for building Info
// without the caller needing to know it. Returns "127.0.0.1" if none is
// found (e.g. no network), which at least keeps pairing usable on the
// same machine for testing.
func LocalIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "127.0.0.1"
	}
	return addr.IP.String()
}
