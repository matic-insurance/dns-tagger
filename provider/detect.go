package provider

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// Provider kinds returned by DetectProvider and accepted by the factory.
const (
	KindCloudflare = "cloudflare"
	KindDNSimple   = "dnsimple"
)

// DetectProvider resolves the authoritative nameservers for a zone and maps
// them to a known DNS provider kind. It returns an error when the nameservers
// don't match any supported provider so the caller can surface a clear,
// per-zone message.
func DetectProvider(ctx context.Context, zone string, timeout time.Duration) (string, error) {
	lookupCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		lookupCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	nameservers, err := net.DefaultResolver.LookupNS(lookupCtx, zone)
	if err != nil {
		return "", fmt.Errorf("could not resolve NS records for zone '%s': %w", zone, err)
	}

	hosts := make([]string, 0, len(nameservers))
	for _, ns := range nameservers {
		hosts = append(hosts, ns.Host)
	}

	kind := classifyNameservers(hosts)
	if kind == "" {
		return "", fmt.Errorf("could not detect DNS provider for zone '%s' from nameservers %v; use --provider to set it manually", zone, hosts)
	}
	return kind, nil
}

// classifyNameservers maps a set of nameserver hostnames to a provider kind.
// It is pure (no DNS) so it can be unit-tested directly.
func classifyNameservers(hosts []string) string {
	for _, host := range hosts {
		h := strings.ToLower(strings.TrimSuffix(host, "."))
		switch {
		case strings.Contains(h, "ns.cloudflare.com"):
			return KindCloudflare
		case strings.Contains(h, "dnsimple.com"):
			return KindDNSimple
		}
	}
	return ""
}
