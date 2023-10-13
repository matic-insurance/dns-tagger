package registry

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestZone_IsManagingEndpoint(t *testing.T) {
	zone := NewZone("dummy.host")
	tests := []struct {
		name     string
		endpoint Endpoint
		want     bool
	}{
		{
			name:     "Apex Endpoint",
			endpoint: Endpoint{Host: "dummy.host"},
			want:     true,
		},
		{
			name:     "Subdomain",
			endpoint: Endpoint{Host: "test.dummy.host"},
			want:     true,
		},
		{
			name:     "Deep level domain",
			endpoint: Endpoint{Host: "webserver.test.dummy.host"},
			want:     true,
		},
		{
			name:     "Another top level",
			endpoint: Endpoint{Host: "webserver.dummy.com"},
			want:     false,
		},
		{
			name:     "Another 2nd level",
			endpoint: Endpoint{Host: "webserver.example.host"},
			want:     false,
		},
		{
			name:     "Contain zone name",
			endpoint: Endpoint{Host: "dummy.host.example.com"},
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, zone.IsManagingEndpoint(&tt.endpoint), "IsManagingEndpoint(%v)", tt.endpoint)
		})
	}
}
