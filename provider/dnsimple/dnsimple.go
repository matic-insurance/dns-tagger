package dnsimple

import (
	"context"
	"fmt"
	"github.com/dnsimple/dnsimple-go/dnsimple"
	"github.com/matic-insurance/external-dns-dialer/provider"
	"github.com/matic-insurance/external-dns-dialer/registry"
	"golang.org/x/oauth2"
	"os"
	"strconv"
	"strings"
)

type dnsimpleProvider struct {
	provider.BaseProvider
	client    *dnsimple.ZonesService
	identity  *dnsimple.IdentityService
	accountID string
	zones     []string
}

func (p dnsimpleProvider) Whoami() string {
	return fmt.Sprintf("DNSimple for Account %s", p.accountID)
}

func NewDnsimpleProvider(zones []string) (provider.Provider, error) {
	oauthToken := os.Getenv("DNSIMPLE_OAUTH")
	if len(oauthToken) == 0 {
		return nil, fmt.Errorf("no dnsimple authentication provided provided (DNSIMPLE_OAUTH or DNSIMPLE_ACCOUNT/DNSIMPLE_TOKEN are missing)")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: oauthToken})
	tc := oauth2.NewClient(context.Background(), ts)

	client := dnsimple.NewClient(tc)
	client.SetUserAgent(fmt.Sprintf("Kubernetes ExternalDNS Dialer"))

	providerInstance := &dnsimpleProvider{
		client:   client.Zones,
		identity: client.Identity,
		zones:    zones,
	}

	whoamiResponse, err := providerInstance.identity.Whoami(context.Background())
	if err != nil {
		return nil, err
	}
	providerInstance.accountID = int64ToString(whoamiResponse.Data.Account.ID)
	return providerInstance, nil
}

func (p dnsimpleProvider) ReadZones() ([]*registry.Zone, error) {
	zones := make([]*registry.Zone, 0)
	for _, zone := range p.zones {
		currentZone := registry.NewZone(zone)
		hostRecords := make([]*registry.Host, 0)
		registryRecords := make([]*registry.Record, 0)
		page := 1
		listOptions := &dnsimple.ZoneRecordListOptions{}
		for {
			listOptions.ListOptions.Page = &page
			dnsRecords, err := p.client.ListRecords(context.Background(), p.accountID, zone, listOptions)
			if err != nil {
				return nil, err
			}
			for _, dnsRecord := range dnsRecords.Data {
				name := fmt.Sprintf("%s.%s", dnsRecord.Name, dnsRecord.ZoneID)
				if currentZone.IsRegistryRecordType(dnsRecord.Type) {
					info := strings.Trim(dnsRecord.Content, "\"")
					if strings.HasPrefix(info, registry.ExternalDnsIdentifier) {
						registryRecords = append(registryRecords, registry.NewRecord(name, info))
					}
				} else if currentZone.IsHostRecordType(dnsRecord.Type) {
					hostRecords = append(hostRecords, registry.NewHost(name, dnsRecord.Type, dnsRecord.Content))
				}
			}
			page++
			if page > dnsRecords.Pagination.TotalPages {
				break
			}
		}

		for _, hostRecord := range hostRecords {
			for _, registryRecord := range registryRecords {
				if registryRecord.IsManaging(hostRecord) {
					hostRecord.AddRegistryRecord(registryRecord)
				}
			}
			currentZone.AddHost(hostRecord)
		}
		zones = append(zones, currentZone)
	}
	return zones, nil
}

func int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}
