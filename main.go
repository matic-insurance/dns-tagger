package main

import (
	"context"
	"github.com/matic-insurance/external-dns-dialer/pkg"
	"github.com/matic-insurance/external-dns-dialer/provider"
	"github.com/matic-insurance/external-dns-dialer/provider/dnsimple"
	"github.com/matic-insurance/external-dns-dialer/registry"
	"github.com/matic-insurance/external-dns-dialer/source"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := initConfig()

	ctx, cancel := context.WithCancel(context.Background())
	go handleSigterm(cancel)

	registryRecords, dnsProvider := getZones(cfg)
	sourceEndpoints := getSourceEndpoints(ctx, cfg)
	selector := pkg.NewSelector(cfg, dnsProvider)
	configureNewOwner(selector, sourceEndpoints, registryRecords)
}

func configureNewOwner(selector *pkg.Selector, endpoints []*registry.Endpoint, zones []*registry.Zone) {
	updatedRecords, err := selector.ClaimEndpointsOwnership(endpoints, zones)
	if err != nil {
		log.Fatalf("Owner updates aborted: %s", err)
	}
	log.Infof("Finished updating registry records. Updated '%d' records", updatedRecords)
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

func getSourceEndpoints(ctx context.Context, cfg *pkg.Config) []*registry.Endpoint {
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

	log.Info("Fetching source endpoints")

	var endpoints []*registry.Endpoint
	for _, endpointsSource := range sources {
		sourceEndpoints, err := endpointsSource.Endpoints(ctx)

		if err != nil {
			log.Fatal(err)
		}

		endpoints = append(endpoints, sourceEndpoints...)
	}
	return endpoints
}

func getZones(cfg *pkg.Config) ([]*registry.Zone, provider.Provider) {
	dnsProvider, err := dnsimple.NewDnsimpleProvider(cfg, []string{"matic.link"})
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Fetching registry records")
	zones, err := dnsProvider.ReadZones()
	if err != nil {
		log.Fatal(err)
	}

	allHosts := make([]*registry.Host, 0)
	for _, zone := range zones {
		allHosts = append(allHosts, zone.Hosts...)
	}

	return zones, dnsProvider
}
