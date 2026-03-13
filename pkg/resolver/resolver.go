package resolver

import (
	"context"
	"net"
	"strings"

	"github.com/miekg/dns"
)

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

// lookupMDNSHost sends A and AAAA queries to the mDNS multicast address.
func lookupMDNSHost(ctx context.Context, name string) ([]string, error) {
	fqdn := dns.Fqdn(name)
	c := new(dns.Client)
	c.Net = "udp"

	var ips []string

	// Query A records via IPv4 multicast
	m := new(dns.Msg)
	m.SetQuestion(fqdn, dns.TypeA)
	m.RecursionDesired = false
	r, _, err := c.ExchangeContext(ctx, m, "224.0.0.251:5353")
	if err == nil {
		for _, ans := range r.Answer {
			if a, ok := ans.(*dns.A); ok {
				ips = append(ips, a.A.String())
			}
		}
	}

	// Query AAAA records via IPv6 multicast
	m = new(dns.Msg)
	m.SetQuestion(fqdn, dns.TypeAAAA)
	m.RecursionDesired = false
	r, _, err = c.ExchangeContext(ctx, m, "[ff02::fb]:5353")
	if err == nil {
		for _, ans := range r.Answer {
			if aaaa, ok := ans.(*dns.AAAA); ok {
				ips = append(ips, aaaa.AAAA.String())
			}
		}
	}

	if len(ips) == 0 {
		return nil, &net.DNSError{Err: "no mDNS records found", Name: name}
	}
	return ips, nil
}
