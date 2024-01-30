package dnsimple

import (
	"context"
	"fmt"
	"github.com/dnsimple/dnsimple-go/dnsimple"
	"github.com/matic-insurance/dns-tager/pkg"
	"github.com/matic-insurance/dns-tager/provider"
	"github.com/matic-insurance/dns-tager/registry"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"os"
	"strconv"
	"strings"
)

type dnsimpleProvider struct {
	provider.BaseProvider
	cfg       *pkg.Config
	client    dnsimpleZoneServiceApi
	identity  *dnsimple.IdentityService
	accountID string
	zones     []string
}

type dnsimpleZoneServiceApi interface {
	ListZones(ctx context.Context, accountID string, options *dnsimple.ZoneListOptions) (*dnsimple.ZonesResponse, error)
	ListRecords(ctx context.Context, accountID string, zoneID string, options *dnsimple.ZoneRecordListOptions) (*dnsimple.ZoneRecordsResponse, error)
	UpdateRecord(ctx context.Context, accountID string, zoneID string, recordID int64, recordAttributes dnsimple.ZoneRecordAttributes) (*dnsimple.ZoneRecordResponse, error)
}

func (p dnsimpleProvider) Whoami(_ context.Context) string {
	return fmt.Sprintf("DNSimple for Account %s", p.accountID)
}

func NewDnsimpleProvider(cfg *pkg.Config, zones []string) (provider.Provider, error) {
	oauthToken := os.Getenv("DNSIMPLE_OAUTH")
	if len(oauthToken) == 0 {
		return nil, fmt.Errorf("no dnsimple authentication provided provided (DNSIMPLE_OAUTH or DNSIMPLE_ACCOUNT/DNSIMPLE_TOKEN are missing)")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: oauthToken})
	tc := oauth2.NewClient(context.Background(), ts)

	client := dnsimple.NewClient(tc)
	client.SetUserAgent(fmt.Sprintf("Kubernetes ExternalDNS Dialer"))

	providerInstance := &dnsimpleProvider{
		cfg:      cfg,
		client:   client.Zones,
		identity: client.Identity,
		zones:    zones,
	}

	if cfg.AccountId != "" {
		providerInstance.accountID = cfg.AccountId
		return providerInstance, nil
	}

	whoamiResponse, err := providerInstance.identity.Whoami(context.Background())
	if err != nil {
		return nil, err
	}

	if whoamiResponse.Data.Account == nil {
		return nil, fmt.Errorf("can not detected DNSimple accout-id, use --account-id to specify it manually")
	}

	providerInstance.accountID = int64ToString(whoamiResponse.Data.Account.ID)
	return providerInstance, nil
}

func (p dnsimpleProvider) ReadZones(ctx context.Context) ([]*registry.Zone, error) {
	zones := make([]*registry.Zone, 0)
	for _, zone := range p.zones {
		currentZone := registry.NewZone(zone)
		hostRecords := make([]*registry.Host, 0)
		registryRecords := make([]*registry.Record, 0)
		page := 1
		listOptions := &dnsimple.ZoneRecordListOptions{}
		for {
			listOptions.ListOptions.Page = &page
			dnsRecords, err := p.client.ListRecords(ctx, p.accountID, zone, listOptions)
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

func (p dnsimpleProvider) UpdateRegistryRecord(ctx context.Context, zone *registry.Zone, record *registry.Record) (int, error) {
	if p.cfg.Apply {
		recordID, err := p.getRecordID(ctx, zone, record.Name)
		if err != nil {
			return 0, err
		}
		_, err = p.client.UpdateRecord(ctx, p.accountID, zone.Name, recordID, dnsimple.ZoneRecordAttributes{Content: record.Info()})
		if err != nil {
			return 0, err
		}
		return 1, nil
	} else {
		log.Infof("Dry Run: Updated %s registry value to %s", record.Name, record.Info())
		return 1, nil
	}
}

func (p dnsimpleProvider) getRecordID(ctx context.Context, zone *registry.Zone, recordName string) (recordID int64, err error) {
	page := 1
	if recordName == zone.Name {
		recordName = "" // Apex records have an empty name
	} else {
		recordName = strings.TrimSuffix(recordName, fmt.Sprintf(".%s", zone.Name))
	}

	listOptions := &dnsimple.ZoneRecordListOptions{Name: &recordName}
	for {
		listOptions.Page = &page
		records, err := p.client.ListRecords(ctx, p.accountID, zone.Name, listOptions)
		if err != nil {
			return 0, err
		}

		for _, record := range records.Data {
			if record.Name == recordName {
				return record.ID, nil
			}
		}

		page++
		if page > records.Pagination.TotalPages {
			break
		}
	}
	return 0, fmt.Errorf("no record id found")
}

func int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}
