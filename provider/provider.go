package provider

import "github.com/matic-insurance/external-dns-dialer/registry"

type Provider interface {
	Whoami() string
	ReadZones() ([]*registry.Zone, error)
	UpdateRegistryRecord(zone *registry.Zone, record *registry.Record) (updatedRecords int, err error)
}

type BaseProvider struct{}
