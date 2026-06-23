package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matic-insurance/dns-tager/pkg"
	"github.com/matic-insurance/dns-tager/provider"
	"github.com/matic-insurance/dns-tager/provider/cloudflare"
	"github.com/matic-insurance/dns-tager/provider/dnsimple"
	"github.com/matic-insurance/dns-tager/registry"
	"github.com/matic-insurance/dns-tager/source"
	log "github.com/sirupsen/logrus"
)

func main() {
	cfg := initConfig()
	registry.Prefix = cfg.TXTPrefix
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
		Labels:         cfg.Labels,
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
	dnsProvider, err := buildProvider(ctx, cfg)
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

// buildProvider determines the DNS provider for each configured zone (either
// auto-detected from NS records or forced via --provider), groups the zones by
// provider, and returns a single provider. When zones span more than one
// provider it returns a RoutingProvider that dispatches per zone.
func buildProvider(ctx context.Context, cfg *pkg.Config) (provider.Provider, error) {
	// Preserve zone order while grouping by provider kind.
	groups := make(map[string][]string)
	var order []string
	for _, zone := range cfg.DNSZones {
		kind := cfg.Provider
		if kind == "" || kind == "auto" {
			detected, err := provider.DetectProvider(ctx, zone, cfg.RequestTimeout)
			if err != nil {
				return nil, err
			}
			kind = detected
		}
		log.Infof("zone %s → %s", zone, kind)
		if _, seen := groups[kind]; !seen {
			order = append(order, kind)
		}
		groups[kind] = append(groups[kind], zone)
	}

	byZone := make(map[string]provider.Provider)
	var all []provider.Provider
	for _, kind := range order {
		zones := groups[kind]
		var p provider.Provider
		var err error
		switch kind {
		case provider.KindCloudflare:
			p, err = cloudflare.NewCloudflareProvider(cfg, zones)
		case provider.KindDNSimple:
			p, err = dnsimple.NewDnsimpleProvider(cfg, zones)
		default:
			return nil, fmt.Errorf("unsupported DNS provider '%s'", kind)
		}
		if err != nil {
			return nil, err
		}
		for _, zone := range zones {
			byZone[zone] = p
		}
		all = append(all, p)
	}

	if len(all) == 1 {
		return all[0], nil
	}
	return provider.NewRoutingProvider(byZone, all), nil
}
