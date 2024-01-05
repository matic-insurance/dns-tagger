package pkg

import (
	"context"
	"github.com/matic-insurance/dns-tager/provider"
	"github.com/matic-insurance/dns-tager/registry"
	log "github.com/sirupsen/logrus"
)

type Selector struct {
	cfg      *Config
	provider provider.Provider
}

func NewSelector(cfg *Config, provider provider.Provider) *Selector {
	return &Selector{cfg: cfg, provider: provider}
}

func (s *Selector) ClaimEndpointsOwnership(ctx context.Context, endpoints []*registry.Endpoint, zones []*registry.Zone) (updatedRecords int, err error) {
	for _, endpoint := range endpoints {
		log.Debugf("Processing '%s'", endpoint)
		zone := findEndpointZone(endpoint, zones)
		if zone == nil {
			log.Warnf("Can't find DNS zone information for '%s'", endpoint)
			continue
		}
		newUpdatedRecords, err := s.claimEndpoint(ctx, endpoint, zone)
		if err != nil {
			return updatedRecords, err
		}
		updatedRecords += newUpdatedRecords
	}
	return updatedRecords, nil
}

func (s *Selector) ClaimEndpointsResource(ctx context.Context, endpoints []*registry.Endpoint, zones []*registry.Zone) (updatedRecords int, err error) {
	for _, endpoint := range endpoints {
		log.Debugf("Processing '%s'", endpoint)
		zone := findEndpointZone(endpoint, zones)
		if zone == nil {
			log.Warnf("Can't find DNS zone information for '%s'", endpoint)
			continue
		}
		newUpdatedRecords, err := s.claimEndpointResource(ctx, endpoint, zone)
		if err != nil {
			return updatedRecords, err
		}
		updatedRecords += newUpdatedRecords
	}
	return updatedRecords, nil
}

func (s *Selector) claimEndpoint(ctx context.Context, endpoint *registry.Endpoint, zone *registry.Zone) (updatedRecords int, err error) {
	hostDiscovered := false
	for _, host := range zone.Hosts {
		if endpoint.Host == host.Name {
			log.Debugf("Host record found for '%s'", endpoint)
			hostDiscovered = true
			if host.IsManaged() {
				for _, registryRecord := range host.RegistryRecords {
					if s.isAlreadyOwned(registryRecord.Owner) {
						log.Debugf("Owner info up to date for '%s'", registryRecord)
						continue
					}
					if !s.isAllowedOwner(registryRecord.Owner) {
						log.Warnf("Owner not updated. Unsupported previous owner. '%s'", registryRecord.Owner)
						continue
					}

					log.Infof("Updating owner info for '%s' to '%s'", registryRecord, s.cfg.CurrentOwnerID)
					updatedRecord := registryRecord.ClaimOwnership(s.cfg.CurrentOwnerID, endpoint.Resource)
					updates, err := s.provider.UpdateRegistryRecord(ctx, zone, updatedRecord)
					updatedRecords += updates
					if err != nil {
						return updatedRecords, err
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

func (s *Selector) claimEndpointResource(ctx context.Context, endpoint *registry.Endpoint, zone *registry.Zone) (updatedRecords int, err error) {
	hostDiscovered := false
	for _, host := range zone.Hosts {
		if endpoint.Host == host.Name {
			log.Debugf("RRR Host record found for '%s'", endpoint)
			hostDiscovered = true
			if host.IsManaged() {
				for _, registryRecord := range host.RegistryRecords {
					log.Debugf("host.Name: '%s'", host.Name)
					log.Debugf("Resource DNSimple: '%s'", registryRecord.Resource)
					log.Debugf("Resource K8S: '%s'", endpoint.Resource)

					if ( registryRecord.Resource != endpoint.Resource ) {
						log.Debug("!!!!!!!!!!!!!!!!! UPDATE !!!!!!!!!!!!!!!!!")
						updatedRecord := registryRecord.ClaimResource(s.cfg.CurrentOwnerID, endpoint.Resource)
						//updates, err := s.provider.UpdateRegistryRecord(ctx, zone, updatedRecord)
					}


					//log.Debugf("Resource: '%s'", s.Resource)
		// 			if s.isAlreadyOwned(registryRecord.Owner) {
		// 				log.Debugf("Resource info up to date for '%s'", registryRecord)
		// 				continue
		// 			}
		// 			// if !s.isAllowedOwner(registryRecord.Owner) {
		// 			// 	log.Warnf("Resource not updated. Unsupported previous owner. '%s'", registryRecord.Owner)
		// 			// 	continue
		// 			// }

		// 			log.Infof("Updating Resource info for '%s' to '%s'", registryRecord, s.cfg.CurrentOwnerID)
		// 			updatedRecord := registryRecord.ClaimOwnership(s.cfg.CurrentOwnerID, endpoint.Resource)
		// 			//updates, err := s.provider.UpdateRegistryRecord(ctx, zone, updatedRecord)
		// 			updatedRecords += updates
		// 			if err != nil {
		// 				return updatedRecords, err
		// 			}
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

func (s *Selector) isAlreadyOwned(owner string) bool {
	return owner == s.cfg.CurrentOwnerID
}

func (s *Selector) isAllowedOwner(owner string) bool {
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
