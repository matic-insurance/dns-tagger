package provider

import "github.com/matic-insurance/external-dns-dialer/registry"

type Provider interface {
	Whoami() string
	AllRegistryRecords() (map[registry.Zone][]registry.Record, error)
}
