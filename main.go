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
	records := getRecords()
	endpoints := getEndpoints(ctx, cfg)
	printUncontrolledRecords(records, endpoints)
}

func printUncontrolledRecords(records []registry.Record, endpoints []registry.Endpoint) {
	log.Infoln("List of registry records")
	for _, dnsRecord := range records {
		log.Infoln("Record: ", dnsRecord.Name, dnsRecord.Resource)
	}
	log.Infoln("List of endpoints")
	for _, managedEndpoint := range endpoints {
		log.Infoln("Endpoint: ", managedEndpoint.Host, managedEndpoint.Resource)
	}
	for _, managedEndpoint := range endpoints {
		ownedByRegistry := false
		if !strings.HasSuffix(managedEndpoint.Host, "matic.link") {
			continue
		}
		for _, dnsRecord := range records {
			if managedEndpoint.Resource == dnsRecord.Resource {
				ownedByRegistry = true
				if strings.HasPrefix(dnsRecord.Name, "k8s-staging") {
					if dnsRecord.Owner == "matic" {
						continue
					} else {
						log.Warnf("Cluster resource %s owned by another instance %s\n", managedEndpoint.Resource, dnsRecord.Owner)
					}
				} else {
					log.Warnf("Cluster resource %s owned by deprecated instance %s:%s\n", managedEndpoint.Resource, dnsRecord.Owner, dnsRecord.Name)
					continue
				}
			}
		}
		if !ownedByRegistry {
			log.Warnf("Missing registry for %s for %s dns\n", managedEndpoint.Resource, managedEndpoint.Host)
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

func getEndpoints(ctx context.Context, cfg *pkg.Config) []registry.Endpoint {
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

func getRecords() []registry.Record {
	dnsimpleProvider, err := dnsimple.NewDnsimpleProvider([]registry.Zone{registry.NewZone("matic.link")})
	if err != nil {
		log.Fatal(err)
	}

	log.Infoln("Fetching registry records")
	recordsMap, err := dnsimpleProvider.AllRegistryRecords()
	if err != nil {
		log.Fatal(err)
	}

	var allRecords []registry.Record
	for _, zoneRecords := range recordsMap {
		allRecords = append(allRecords, zoneRecords...)
	}

	return allRecords
}
