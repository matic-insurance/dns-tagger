package main

import (
	"context"
	"github.com/matic-insurance/external-dns-dialer/pkg"
	"github.com/matic-insurance/external-dns-dialer/provider/dnsimple"
	"github.com/matic-insurance/external-dns-dialer/registry"
	"github.com/matic-insurance/external-dns-dialer/source"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	cfg := initConfig()

	ctx, cancel := context.WithCancel(context.Background())
	go handleSigterm(cancel)
	registryRecords := getHosts(cfg)
	sourceEndpoints := getSourceEndpoints(ctx, cfg)
	printUncontrolledRecords(cfg, registryRecords, sourceEndpoints)
}

func printUncontrolledRecords(cfg *pkg.Config, hosts []*registry.Host, sourceEndpoints []registry.Endpoint) {
	log.Debugln("List of Hosts")
	for _, host := range hosts {
		log.Debugln("Discovered Host: ", host)
	}
	log.Debugln("List of source endpoints")
	for _, sourceEndpoint := range sourceEndpoints {
		log.Debugln("Endpoint: ", sourceEndpoint.Host, sourceEndpoint.Resource)
	}

	supportedOwners := strings.Join(cfg.PreviousOwnerIDs, ",")
	for _, sourceEndpoint := range sourceEndpoints {
		if !strings.HasSuffix(sourceEndpoint.Host, "matic.link") {
			continue
		}
		ownedByRegistry := false
		for _, host := range hosts {
			if sourceEndpoint.Host == host.Name {
				if host.IsManaged() {
					ownedByRegistry = true
					for _, registryRecord := range host.RegistryRecords {
						if registryRecord.Owner != cfg.CurrentOwnerID {
							if strings.Contains(supportedOwners, registryRecord.Owner) {
								log.Warnf("Registry owner should be updated for %s\n", registryRecord)
							} else {
								log.Warnf("Unsupported registry owner, cannot update %s\n", registryRecord)
							}
						}
						if registryRecord.Resource != sourceEndpoint.Resource {
							log.Warnf("Wrong resource information %s\n", registryRecord)
						}
					}
				} else {
					log.Warnf("Missing registry for %s for %s dns\n", sourceEndpoint.Resource, sourceEndpoint.Host)
				}
			}
		}
		if !ownedByRegistry {
			log.Warnf("Missing host record for %s for %s dns\n", sourceEndpoint.Resource, sourceEndpoint.Host)
		}
	}
}

func initConfig() *pkg.Config {
	cfg := pkg.NewConfig()
	if err := cfg.ParseFlags(os.Args[1:]); err != nil {
		log.Fatalf("flag parsing error: %v", err)
	}
	if cfg.LogFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}
	log.Infof("config: %s", cfg)

	ll, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatalf("failed to parse log level: %v", err)
	}
	log.SetLevel(ll)

	return cfg
}

func handleSigterm(cancel func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	<-signals
	log.Info("Received SIGTERM. Terminating...")
	cancel()
}

func getSourceEndpoints(ctx context.Context, cfg *pkg.Config) []registry.Endpoint {
	// Create a source.Config from the flags passed by the user.
	sourceCfg := &source.Config{
		Namespace:      cfg.Namespace,
		KubeConfig:     cfg.KubeConfig,
		APIServerURL:   cfg.APIServerURL,
		RequestTimeout: cfg.RequestTimeout,
		UpdateEvents:   cfg.UpdateEvents,
	}
	sources, err := source.ByNames(ctx, &source.SingletonClientGenerator{
		KubeConfig:   cfg.KubeConfig,
		APIServerURL: cfg.APIServerURL,
		// If update events are enabled, disable timeout.
		RequestTimeout: func() time.Duration {
			if cfg.UpdateEvents {
				return 0
			}
			return cfg.RequestTimeout
		}(),
	}, cfg.Sources, sourceCfg)

	if err != nil {
		log.Fatal(err)
	}

	log.Infoln("Fetching source endpoints")

	var endpoints []registry.Endpoint
	for _, endpointsSource := range sources {
		sourceEndpoints, err := endpointsSource.Endpoints(ctx)

		if err != nil {
			log.Fatal(err)
		}

		endpoints = append(endpoints, sourceEndpoints...)
	}
	return endpoints
}

func getHosts(*pkg.Config) []*registry.Host {
	dnsimpleProvider, err := dnsimple.NewDnsimpleProvider([]string{"matic.link"})
	if err != nil {
		log.Fatal(err)
	}

	log.Infoln("Fetching registry records")
	zones, err := dnsimpleProvider.ReadZones()
	if err != nil {
		log.Fatal(err)
	}

	allHosts := make([]*registry.Host, 0)
	for _, zone := range zones {
		allHosts = append(allHosts, zone.Hosts...)
	}

	return allHosts
}
