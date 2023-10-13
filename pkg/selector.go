package pkg

import (
	"github.com/matic-insurance/external-dns-dialer/provider"
	"github.com/matic-insurance/external-dns-dialer/registry"
	log "github.com/sirupsen/logrus"
)

type Selector struct {
	cfg      *Config
	provider provider.Provider
}

func NewSelector(cfg *Config, provider provider.Provider) *Selector {
	return &Selector{cfg: cfg, provider: provider}
}

func (s *Selector) ConfigureNewOwner(endpoints []*registry.Endpoint, zones []*registry.Zone) (updatedRecords int, err error) {
	for _, endpoint := range endpoints {
		log.Debugf("Processing '%s'", endpoint)
		zone := findEndpointZone(endpoint, zones)
		if zone == nil {
			log.Warnf("Can't find DNS zone information for '%s'", endpoint)
			continue
		}
		newUpdatedRecords, err := s.UpdateRegistryRecords(endpoint, zone)
		if err != nil {
			return updatedRecords, err
		}
		updatedRecords += newUpdatedRecords
	}
	return updatedRecords, nil
}

func (s *Selector) UpdateRegistryRecords(endpoint *registry.Endpoint, zone *registry.Zone) (updatedRecords int, err error) {
	hostDiscovered := false
	for _, host := range zone.Hosts {
		if endpoint.Host == host.Name {
			log.Debugf("Host record found for '%s'", endpoint)
			hostDiscovered = true
			if host.IsManaged() {
				for _, registryRecord := range host.RegistryRecords {
					updatedRecord := registryRecord
					if registryRecord.Owner == s.cfg.CurrentOwnerID {
						log.Debugf("Owner info up to date for '%s'", registryRecord)
					} else {
						if s.isPreviousOwnerAllowed(registryRecord.Owner) {
							log.Infof("Updating owner info for '%s' to '%s'", registryRecord, s.cfg.CurrentOwnerID)
							updatedRecord = updatedRecord.NewOwner(s.cfg.CurrentOwnerID)
						} else {
							log.Warnf("Owner not updated. Unsupported previous owner. '%s'", registryRecord.Owner)
						}
					}
					if registryRecord.Resource != endpoint.Resource {
						log.Infof("Updating resource info for '%s' to '%s'", registryRecord, endpoint.Resource)
						updatedRecord = updatedRecord.NewResource(endpoint.Resource)
					}
					if registryRecord != updatedRecord {
						updates, err := s.provider.UpdateRegistryRecord(zone, updatedRecord)
						if err != nil {
							return updatedRecords, err
						}
						updatedRecords += updates
					}
				}
			} else {
				log.Warnf("Missing registry records for '%s'", endpoint)
			}
		}
	}
	if !hostDiscovered {
		log.Warnf("Missing host record for '%s'", endpoint)
	}
	return updatedRecords, nil
}

func (s *Selector) isPreviousOwnerAllowed(owner string) bool {
	for _, previousOwner := range s.cfg.PreviousOwnerIDs {
		if previousOwner == owner {
			return true
		}
	}
	return false
}

func findEndpointZone(endpoint *registry.Endpoint, zones []*registry.Zone) *registry.Zone {
	for _, zone := range zones {
		if zone.IsManagingEndpoint(endpoint) {
			return zone
		}
	}
	return nil
}
