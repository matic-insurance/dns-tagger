package provider

import "github.com/matic-insurance/external-dns-dialer/registry"

type Provider interface {
	Whoami() string
	ReadZones() ([]*registry.Zone, error)
}

type BaseProvider struct{}
