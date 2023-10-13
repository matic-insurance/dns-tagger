package provider

import (
	"context"
	"github.com/matic-insurance/external-dns-dialer/registry"
)

type Provider interface {
	Whoami(ctx context.Context) string
	ReadZones(ctx context.Context) ([]*registry.Zone, error)
	UpdateRegistryRecord(ctx context.Context, zone *registry.Zone, record *registry.Record) (updatedRecords int, err error)
}

type BaseProvider struct{}
