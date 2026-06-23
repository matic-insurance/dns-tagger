package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/matic-insurance/dns-tager/registry"
)

// RoutingProvider dispatches operations to the concrete provider responsible
// for a given zone. It lets a single run span zones hosted on different DNS
// providers (e.g. some on Cloudflare, some on DNSimple).
type RoutingProvider struct {
	BaseProvider
	byZone map[string]Provider // zone name -> owning provider
	all    []Provider          // unique provider instances
}

// NewRoutingProvider builds a routing provider. byZone maps each zone name to
// its owning provider; all holds the unique provider instances (each appears
// once, so ReadZones isn't invoked twice for providers owning multiple zones).
func NewRoutingProvider(byZone map[string]Provider, all []Provider) *RoutingProvider {
	return &RoutingProvider{byZone: byZone, all: all}
}

func (r *RoutingProvider) Whoami(ctx context.Context) string {
	parts := make([]string, 0, len(r.all))
	for _, p := range r.all {
		parts = append(parts, p.Whoami(ctx))
	}
	return strings.Join(parts, ", ")
}

func (r *RoutingProvider) ReadZones(ctx context.Context) ([]*registry.Zone, error) {
	zones := make([]*registry.Zone, 0)
	for _, p := range r.all {
		providerZones, err := p.ReadZones(ctx)
		if err != nil {
			return nil, err
		}
		zones = append(zones, providerZones...)
	}
	return zones, nil
}

func (r *RoutingProvider) UpdateRegistryRecord(ctx context.Context, zone *registry.Zone, record *registry.Record) (int, error) {
	p, ok := r.byZone[zone.Name]
	if !ok {
		return 0, fmt.Errorf("no DNS provider registered for zone '%s'", zone.Name)
	}
	return p.UpdateRegistryRecord(ctx, zone, record)
}
