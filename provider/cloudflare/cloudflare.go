package cloudflare

import (
	"context"
	"fmt"
	"os"
	"strings"

	cf "github.com/cloudflare/cloudflare-go"
	"github.com/matic-insurance/dns-tager/pkg"
	"github.com/matic-insurance/dns-tager/provider"
	"github.com/matic-insurance/dns-tager/registry"
	log "github.com/sirupsen/logrus"
)

type cloudflareProvider struct {
	provider.BaseProvider
	cfg     *pkg.Config
	api     cloudflareDNSApi
	zones   []string
	zoneIDs map[string]string // zone name -> cloudflare zone id (lazily cached)
}

// cloudflareDNSApi is the subset of the cloudflare-go API used by the provider.
// It exists so tests can mock the upstream client. *cloudflare.API satisfies it.
type cloudflareDNSApi interface {
	ZoneIDByName(zoneName string) (string, error)
	ListDNSRecords(ctx context.Context, rc *cf.ResourceContainer, params cf.ListDNSRecordsParams) ([]cf.DNSRecord, *cf.ResultInfo, error)
	UpdateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.UpdateDNSRecordParams) (cf.DNSRecord, error)
}

func (p *cloudflareProvider) Whoami(_ context.Context) string {
	return "Cloudflare"
}

func NewCloudflareProvider(cfg *pkg.Config, zones []string) (provider.Provider, error) {
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	if len(apiToken) == 0 {
		return nil, fmt.Errorf("no cloudflare authentication provided (CLOUDFLARE_API_TOKEN is missing)")
	}

	api, err := cf.NewWithAPIToken(apiToken)
	if err != nil {
		return nil, err
	}

	return &cloudflareProvider{
		cfg:     cfg,
		api:     api,
		zones:   zones,
		zoneIDs: make(map[string]string),
	}, nil
}

func (p *cloudflareProvider) ReadZones(ctx context.Context) ([]*registry.Zone, error) {
	zones := make([]*registry.Zone, 0)
	for _, zone := range p.zones {
		currentZone := registry.NewZone(zone)
		hostRecords := make([]*registry.Host, 0)
		registryRecords := make([]*registry.Record, 0)

		zoneID, err := p.zoneID(zone)
		if err != nil {
			return nil, err
		}

		// ListDNSRecords auto-paginates when neither Page nor PerPage is set.
		dnsRecords, _, err := p.api.ListDNSRecords(ctx, cf.ZoneIdentifier(zoneID), cf.ListDNSRecordsParams{})
		if err != nil {
			return nil, err
		}

		for _, dnsRecord := range dnsRecords {
			// Cloudflare returns the record name already as an FQDN.
			name := dnsRecord.Name
			if currentZone.IsRegistryRecordType(dnsRecord.Type) {
				info := strings.Trim(dnsRecord.Content, "\"")
				if strings.HasPrefix(info, registry.ExternalDnsIdentifier) {
					registryRecords = append(registryRecords, registry.NewRecord(name, info))
				}
			} else if currentZone.IsHostRecordType(dnsRecord.Type) {
				hostRecords = append(hostRecords, registry.NewHost(name, dnsRecord.Type, dnsRecord.Content))
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

func (p *cloudflareProvider) UpdateRegistryRecord(ctx context.Context, zone *registry.Zone, record *registry.Record) (int, error) {
	if p.cfg.Apply {
		zoneID, err := p.zoneID(zone.Name)
		if err != nil {
			return 0, err
		}
		recordID, err := p.getRecordID(ctx, zoneID, record.Name)
		if err != nil {
			return 0, err
		}
		_, err = p.api.UpdateDNSRecord(ctx, cf.ZoneIdentifier(zoneID), cf.UpdateDNSRecordParams{ID: recordID, Content: record.Info()})
		if err != nil {
			return 0, err
		}
		return 1, nil
	} else {
		log.Infof("Dry Run: Updated %s registry value to %s", record.Name, record.Info())
		return 1, nil
	}
}

// getRecordID resolves the cloudflare record id for a TXT registry record by its name.
func (p *cloudflareProvider) getRecordID(ctx context.Context, zoneID string, recordName string) (string, error) {
	records, _, err := p.api.ListDNSRecords(ctx, cf.ZoneIdentifier(zoneID), cf.ListDNSRecordsParams{
		Type: registry.RegistryRecordType,
		Name: recordName,
	})
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if record.Name == recordName {
			return record.ID, nil
		}
	}
	return "", fmt.Errorf("no record id found")
}

// zoneID resolves and caches the cloudflare zone id for a zone name.
func (p *cloudflareProvider) zoneID(zoneName string) (string, error) {
	if id, ok := p.zoneIDs[zoneName]; ok {
		return id, nil
	}
	id, err := p.api.ZoneIDByName(zoneName)
	if err != nil {
		return "", err
	}
	p.zoneIDs[zoneName] = id
	return id, nil
}
