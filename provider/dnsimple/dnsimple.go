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
	provider.Provider
	client    *dnsimple.ZonesService
	identity  *dnsimple.IdentityService
	accountID string
	zones     []registry.Zone
}

func (p dnsimpleProvider) Whoami() string {
	return fmt.Sprintf("DNSimple for Account %s", p.accountID)
}

func (p dnsimpleProvider) AllRegistryRecords() (map[registry.Zone][]registry.Record, error) {
	registryRecords := map[registry.Zone][]registry.Record{}
	for _, zone := range p.zones {
		page := 1
		listOptions := &dnsimple.ZoneRecordListOptions{Type: dnsimple.String("TXT")}
		for {
			dnsRecords, err := p.client.ListRecords(context.Background(), p.accountID, zone.Name, listOptions)
			if err != nil {
				return nil, err
			}
			for _, dnsRecord := range dnsRecords.Data {
				info := strings.Trim(dnsRecord.Content, "\"")
				if strings.HasPrefix(info, registry.ExternalDnsIdentifier) {
					name := fmt.Sprintf("%s.%s", dnsRecord.Name, dnsRecord.ZoneID)
					registryRecords[zone] = append(registryRecords[zone], registry.NewRecord(zone, name, info))
				}
			}
			page++
			if page > dnsRecords.Pagination.TotalPages {
				break
			}
		}
	}
	return registryRecords, nil
}

func NewDnsimpleProvider(zones []registry.Zone) (provider.Provider, error) {
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

func int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}
