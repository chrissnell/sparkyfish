package resolver

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const mdnsTimeout = 2 * time.Second

var mdnsIPv4Addr = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// LookupHost resolves a hostname to IP addresses. For .local names, it
// tries mDNS first (multicast to 224.0.0.251:5353) before falling back
// to the system resolver.
func LookupHost(ctx context.Context, name string) ([]string, error) {
	normalized := strings.TrimSuffix(strings.ToLower(name), ".")

	if strings.HasSuffix(normalized, ".local") {
		ips, err := lookupMDNSHost(ctx, name)
		if err == nil && len(ips) > 0 {
			return ips, nil
		}
		return net.DefaultResolver.LookupHost(ctx, name)
	}

	return net.DefaultResolver.LookupHost(ctx, name)
}

// lookupMDNSHost sends an A query to the mDNS IPv4 multicast group using
// an unconnected UDP socket so we can receive the reply from any source IP.
func lookupMDNSHost(ctx context.Context, name string) ([]string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	m.RecursionDesired = false

	packed, err := m.Pack()
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(mdnsTimeout)
	}
	conn.SetDeadline(deadline)

	if _, err := conn.WriteTo(packed, mdnsIPv4Addr); err != nil {
		return nil, err
	}

	buf := make([]byte, 1500)
	var ips []string
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			break
		}
		var resp dns.Msg
		if err := resp.Unpack(buf[:n]); err != nil {
			continue
		}
		// Match our query ID
		if resp.Id != m.Id {
			continue
		}
		for _, ans := range resp.Answer {
			if a, ok := ans.(*dns.A); ok {
				ips = append(ips, a.A.String())
			}
		}
		if len(ips) > 0 {
			return ips, nil
		}
	}

	if len(ips) == 0 {
		return nil, &net.DNSError{Err: "no mDNS records found", Name: name}
	}
	return ips, nil
}
