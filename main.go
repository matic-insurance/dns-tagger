package main

import (
	"context"
	"github.com/matic-insurance/dns-tager/pkg"
	"github.com/matic-insurance/dns-tager/provider"
	"github.com/matic-insurance/dns-tager/provider/dnsimple"
	"github.com/matic-insurance/dns-tager/registry"
	"github.com/matic-insurance/dns-tager/source"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := initConfig()
	log.Infof("Running in '%s' mode", cfg.Mode)

	ctx, cancel := context.WithCancel(context.Background())
	go handleSigterm(cancel)

	sourceEndpoints := getSourceEndpoints(ctx, cfg)
	zones, dnsProvider := getZones(ctx, cfg)
	selector := pkg.NewSelector(cfg, dnsProvider)
	if cfg.Mode == "owner" {
	 	configureNewOwner(ctx, cfg, selector, sourceEndpoints, zones)
	} else {
		configureNewResource(ctx, cfg, selector, sourceEndpoints, zones)
	}
}

func configureNewOwner(ctx context.Context, cfg *pkg.Config, selector *pkg.Selector, endpoints []*registry.Endpoint, zones []*registry.Zone) {
	updatedRecords, err := selector.ClaimEndpointsOwnership(ctx, endpoints, zones)
	if err != nil {
		log.Fatalf("Owner updates aborted: %s", err)
	}
	if cfg.Apply {
		log.Infof("Finished updating registry records. Updated '%d' records", updatedRecords)
	} else {
		log.Infof("Finished updating registry records. Updated '%d' records in Dry Run mode", updatedRecords)
	}
}

func configureNewResource(ctx context.Context, cfg *pkg.Config, selector *pkg.Selector, endpoints []*registry.Endpoint, zones []*registry.Zone) {
	updatedRecords, err := selector.ClaimEndpointsResource(ctx, endpoints, zones)
	if err != nil {
		log.Fatalf("Resource updates aborted: %s", err)
	}
	if cfg.Apply {
		log.Infof("Finished updating registry records. Updated '%d' records", updatedRecords)
	} else {
		log.Infof("Finished updating registry records. Updated '%d' records in Dry Run mode", updatedRecords)
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

func getSourceEndpoints(ctx context.Context, cfg *pkg.Config) []*registry.Endpoint {
	// Create a source.Config from the flags passed by the user.
	sourceCfg := &source.Config{
		Namespace:      cfg.Namespace,
		KubeConfig:     cfg.KubeConfig,
		APIServerURL:   cfg.APIServerURL,
		RequestTimeout: cfg.RequestTimeout,
	}
	sources, err := source.ByNames(ctx, &source.SingletonClientGenerator{
		KubeConfig:   cfg.KubeConfig,
		APIServerURL: cfg.APIServerURL,
		// If update events are enabled, disable timeout.
		RequestTimeout: func() time.Duration {
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

func getZones(ctx context.Context, cfg *pkg.Config) ([]*registry.Zone, provider.Provider) {
	dnsProvider, err := dnsimple.NewDnsimpleProvider(cfg, cfg.DNSZones)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Fetching registry records")
	zones, err := dnsProvider.ReadZones(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return zones, dnsProvider
}
