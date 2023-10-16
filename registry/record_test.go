package registry

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestNewRecord_InfoParsed(t *testing.T) {
	record := NewRecord("k8s_api.dummy.zone", "heritage=external-dns,external-dns/owner=matic,external-dns/resource=ingress/staging/matic-console-rails-ingress")
	want := &Record{Name: "k8s_api.dummy.zone", Owner: "matic", Resource: "ingress/staging/matic-console-rails-ingress"}
	if !reflect.DeepEqual(record, want) {
		t.Errorf("Record contents not parsed. Got: %v, want: %v", record, want)
	}
}

func TestNewRecord_EmptyInfo(t *testing.T) {
	record := NewRecord("k8s_api.dummy.zone", "")
	want := &Record{Name: "k8s_api.dummy.zone", Owner: "", Resource: ""}
	if !reflect.DeepEqual(record, want) {
		t.Errorf("Record contents not parsed. Got: %v, want: %v", record, want)
	}
}

func TestRecord_Info(t *testing.T) {
	get := Record{Name: "k8s_api.dummy.zone", Owner: "matic", Resource: "ingress/test/webserver"}.Info()
	want := "heritage=external-dns,external-dns/owner=matic,external-dns/resource=ingress/test/webserver"
	assert.Equal(t, want, get, "Should correctly serialize registry information")
}

func TestRecord_IsManaging(t *testing.T) {
	tests := []struct {
		name   string
		record Record
		host   *Host
		want   bool
	}{
		{
			name:   "Managing exact host",
			record: Record{Name: "webserver.dummy.host"},
			host:   &Host{Name: "webserver.dummy.host"},
			want:   true,
		},
		{
			name:   "Managing with registry prefix",
			record: Record{Name: "some-prefix-webserver.dummy.host"},
			host:   &Host{Name: "webserver.dummy.host"},
			want:   true,
		},
		{
			name:   "Managing with registry suffix",
			record: Record{Name: "webserver-registry.dummy.host"},
			host:   &Host{Name: "webserver.dummy.host"},
			want:   true,
		},
		{
			name:   "Not managing different zone",
			record: Record{Name: "webserver-suffix.dummy.com"},
			host:   &Host{Name: "webserver.dummy.host"},
			want:   false,
		},
		{
			name:   "Not managing different domain",
			record: Record{Name: "prefix-webserver.dummy.host"},
			host:   &Host{Name: "webserver2.dummy.host"},
			want:   false,
		},
		{
			name:   "Not managing deeper level",
			record: Record{Name: "4thlevel.prefix-webserver.dummy.host"},
			host:   &Host{Name: "webserver.dummy.host"},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.record.IsManaging(tt.host), "IsManaging(%v)", tt.host)
		})
	}
}
