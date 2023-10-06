package registry

import (
	"reflect"
	"testing"
)

func TestNewRecord(t *testing.T) {
	zone := NewZone("dummy.zone")
	type args struct {
		zone Zone
		name string
		info string
	}
	tests := []struct {
		name string
		args args
		want Record
	}{
		{
			name: "Empty info",
			args: args{zone: zone, name: "k8s_api.dummy.zone", info: ""},
			want: Record{Name: "k8s_api.dummy.zone", Owner: "", Resource: "", Zone: zone, info: ""},
		},
		{
			name: "Full Info",
			args: args{zone: zone, name: "k8s_api.dummy.zone", info: "heritage=external-dns,external-dns/owner=matic,external-dns/resource=ingress/staging/matic-console-rails-ingress"},
			want: Record{
				Name:     "k8s_api.dummy.zone",
				Zone:     zone,
				Owner:    "matic",
				Resource: "ingress/staging/matic-console-rails-ingress",
				info:     "heritage=external-dns,external-dns/owner=matic,external-dns/resource=ingress/staging/matic-console-rails-ingress",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewRecord(tt.args.zone, tt.args.name, tt.args.info); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewRecord() = %v, want %v", got, tt.want)
			}
		})
	}
}
