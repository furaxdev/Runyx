package pairing

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// WakeOnLAN sends a standard Wake-on-LAN magic packet (6 bytes of
// 0xFF followed by the target MAC address repeated 16 times) as a UDP
// broadcast, to wake a sleeping PC so the Android app can reach it.
// macAddr accepts colon- or hyphen-separated hex (e.g.
// "aa:bb:cc:dd:ee:ff").
func WakeOnLAN(macAddr string, broadcastAddr string) error {
	mac, err := parseMAC(macAddr)
	if err != nil {
		return err
	}
	if broadcastAddr == "" {
		broadcastAddr = "255.255.255.255:9"
	}

	packet := make([]byte, 0, 102)
	packet = append(packet, bytesRepeat(0xFF, 6)...)
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}

	addr, err := net.ResolveUDPAddr("udp", broadcastAddr)
	if err != nil {
		return fmt.Errorf("resolve broadcast address: %w", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("dial broadcast: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	return err
}

func parseMAC(s string) ([]byte, error) {
	s = strings.NewReplacer(":", "", "-", "", ".", "").Replace(s)
	if len(s) != 12 {
		return nil, fmt.Errorf("invalid MAC address %q", s)
	}
	mac, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address %q: %w", s, err)
	}
	return mac, nil
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
