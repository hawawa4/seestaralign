package alpaca

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"
	"strings"
	"time"
)

const DiscoveryPort = 32227

type Found struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// Discover sends the Alpaca discovery probe to target (a broadcast address or
// one host) and collects replies until timeout.
func Discover(target string, timeout time.Duration) ([]Found, error) {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(target, fmt.Sprint(DiscoveryPort)))
	if err != nil {
		return nil, err
	}
	if _, err := conn.WriteToUDP([]byte("alpacadiscovery1"), addr); err != nil {
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	var out []Found
	buf := make([]byte, 1024)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return out, nil // deadline
		}
		var r struct{ AlpacaPort int }
		if json.Unmarshal(buf[:n], &r) != nil || r.AlpacaPort == 0 {
			continue
		}
		f := Found{IP: from.IP.String(), Port: r.AlpacaPort}
		dup := false
		for _, o := range out {
			dup = dup || o == f
		}
		if !dup {
			out = append(out, f)
		}
	}
}

// ResolveHost uses the system resolver, then falls back to our own mDNS query
// for *.local names: containers rarely have nss-mdns configured, and a static
// Go binary never uses it anyway.
func ResolveHost(ctx context.Context, host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if addrs, err := net.DefaultResolver.LookupIPAddr(cctx, host); err == nil {
		for _, a := range addrs {
			if a.IP.To4() != nil {
				return a.IP.String(), nil
			}
		}
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSuffix(host, ".")), ".local") {
		if ip, err := MDNSLookup(host, 2*time.Second); err == nil {
			return ip, nil
		}
	}
	return "", fmt.Errorf("could not resolve %q - enter the Seestar's IP address instead", host)
}

// MDNSLookup resolves name with a one-shot mDNS A query. It asks for a
// unicast reply (QU) and also listens on the multicast group in case the
// responder answers there.
func MDNSLookup(name string, timeout time.Duration) (string, error) {
	name = strings.TrimSuffix(name, ".")
	var q []byte
	id := uint16(rand.IntN(65536))
	q = binary.BigEndian.AppendUint16(q, id)
	q = append(q, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0)
	for _, label := range strings.Split(name, ".") {
		q = append(q, byte(len(label)))
		q = append(q, label...)
	}
	q = append(q, 0, 0, 1, 0x80, 1) // type A, class IN with QU bit

	group := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	results := make(chan string, 2)
	deadline := time.Now().Add(timeout)

	listen := func(conn *net.UDPConn) {
		defer conn.Close()
		_ = conn.SetReadDeadline(deadline)
		buf := make([]byte, 9000)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if ip := parseARecord(buf[:n], name); ip != "" {
				results <- ip
				return
			}
		}
	}
	if uc, err := net.ListenUDP("udp4", nil); err == nil {
		_, _ = uc.WriteToUDP(q, group)
		go listen(uc)
	}
	if mc, err := net.ListenMulticastUDP("udp4", nil, group); err == nil {
		go listen(mc)
	}
	select {
	case ip := <-results:
		return ip, nil
	case <-time.After(time.Until(deadline)):
		return "", fmt.Errorf("no mDNS answer for %s", name)
	}
}

func readName(b []byte, off int) (string, int, bool) {
	var labels []string
	end, jumped := off, false
	for hops := 0; hops < 64; hops++ {
		if off >= len(b) {
			return "", 0, false
		}
		l := int(b[off])
		switch {
		case l&0xC0 == 0xC0:
			if off+1 >= len(b) {
				return "", 0, false
			}
			if !jumped {
				end = off + 2
			}
			off = (l&0x3F)<<8 | int(b[off+1])
			jumped = true
		case l == 0:
			if !jumped {
				end = off + 1
			}
			return strings.Join(labels, "."), end, true
		default:
			if off+1+l > len(b) {
				return "", 0, false
			}
			labels = append(labels, string(b[off+1:off+1+l]))
			off += 1 + l
		}
	}
	return "", 0, false
}

func parseARecord(b []byte, name string) string {
	if len(b) < 12 {
		return ""
	}
	qd := int(binary.BigEndian.Uint16(b[4:]))
	rr := int(binary.BigEndian.Uint16(b[6:])) + int(binary.BigEndian.Uint16(b[8:])) + int(binary.BigEndian.Uint16(b[10:]))
	off := 12
	for i := 0; i < qd; i++ {
		_, o, ok := readName(b, off)
		if !ok {
			return ""
		}
		off = o + 4
	}
	for i := 0; i < rr; i++ {
		rname, o, ok := readName(b, off)
		if !ok || o+10 > len(b) {
			return ""
		}
		typ := binary.BigEndian.Uint16(b[o:])
		rdlen := int(binary.BigEndian.Uint16(b[o+8:]))
		o += 10
		if o+rdlen > len(b) {
			return ""
		}
		if typ == 1 && rdlen == 4 && strings.EqualFold(rname, name) {
			return net.IP(b[o : o+4]).String()
		}
		off = o + rdlen
	}
	return ""
}
