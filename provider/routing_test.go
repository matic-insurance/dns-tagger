package provider

import (
	"context"
	"testing"

	"github.com/matic-insurance/dns-tager/registry"
	"github.com/stretchr/testify/assert"
)

// fakeProvider records which zones it was asked to update.
type fakeProvider struct {
	BaseProvider
	name    string
	updated []string
}

func (f *fakeProvider) Whoami(context.Context) string { return f.name }

func (f *fakeProvider) ReadZones(context.Context) ([]*registry.Zone, error) {
	return []*registry.Zone{{Name: f.name + "-zone"}}, nil
}

func (f *fakeProvider) UpdateRegistryRecord(_ context.Context, zone *registry.Zone, _ *registry.Record) (int, error) {
	f.updated = append(f.updated, zone.Name)
	return 1, nil
}

func TestRoutingProvider_UpdateRoutesByZone(t *testing.T) {
	cfProvider := &fakeProvider{name: "cloudflare"}
	dsProvider := &fakeProvider{name: "dnsimple"}

	byZone := map[string]Provider{
		"a.com": cfProvider,
		"b.com": dsProvider,
	}
	router := NewRoutingProvider(byZone, []Provider{cfProvider, dsProvider})

	_, err := router.UpdateRegistryRecord(context.Background(), &registry.Zone{Name: "a.com"}, &registry.Record{})
	assert.NoError(t, err)
	_, err = router.UpdateRegistryRecord(context.Background(), &registry.Zone{Name: "b.com"}, &registry.Record{})
	assert.NoError(t, err)

	assert.Equal(t, []string{"a.com"}, cfProvider.updated, "a.com routed to cloudflare")
	assert.Equal(t, []string{"b.com"}, dsProvider.updated, "b.com routed to dnsimple")
}

func TestRoutingProvider_UpdateUnknownZone(t *testing.T) {
	router := NewRoutingProvider(map[string]Provider{}, nil)
	_, err := router.UpdateRegistryRecord(context.Background(), &registry.Zone{Name: "missing.com"}, &registry.Record{})
	assert.Error(t, err)
}

func TestRoutingProvider_ReadZonesConcatenates(t *testing.T) {
	cfProvider := &fakeProvider{name: "cloudflare"}
	dsProvider := &fakeProvider{name: "dnsimple"}
	router := NewRoutingProvider(map[string]Provider{}, []Provider{cfProvider, dsProvider})

	zones, err := router.ReadZones(context.Background())
	assert.NoError(t, err)
	assert.Len(t, zones, 2)
}
