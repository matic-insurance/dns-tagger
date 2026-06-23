package provider

import "testing"

func TestClassifyNameservers(t *testing.T) {
	cases := []struct {
		name  string
		hosts []string
		want  string
	}{
		{"cloudflare", []string{"anya.ns.cloudflare.com.", "rob.ns.cloudflare.com."}, KindCloudflare},
		{"dnsimple", []string{"ns1.dnsimple.com", "ns2.dnsimple.com", "ns3.dnsimple.com"}, KindDNSimple},
		{"cloudflare uppercase", []string{"ANYA.NS.CLOUDFLARE.COM."}, KindCloudflare},
		{"cloudflare wins when first", []string{"anya.ns.cloudflare.com", "ns1.dnsimple.com"}, KindCloudflare},
		{"unknown", []string{"ns-123.awsdns-45.org", "ns-678.awsdns-90.co.uk"}, ""},
		{"empty", nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyNameservers(tc.hosts); got != tc.want {
				t.Errorf("classifyNameservers(%v) = %q, want %q", tc.hosts, got, tc.want)
			}
		})
	}
}
