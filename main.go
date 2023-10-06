package main

import (
	"context"
	"fmt"
	"github.com/matic-insurance/external-dns-dialer/pkg"
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
	//printRecords()
	printEndpoints(ctx, cfg)
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

func printEndpoints(ctx context.Context, cfg *pkg.Config) {
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

	fmt.Println("Fetching source endpoints")

	for _, source := range sources {
		endpoints, err := source.Endpoints(ctx)

		if err != nil {
			log.Fatal(err)
		}

		for _, endpoint := range endpoints {
			fmt.Println(endpoint.Resource)
		}
	}
}

func printRecords() {
	dnsimpleProvider, err := dnsimple.NewDnsimpleProvider([]registry.Zone{registry.NewZone("matic.link")})
	if err != nil {
		fmt.Printf("Cannot init dnSimple: %s\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("%s", dnsimpleProvider.Whoami())
	}

	fmt.Println("Fetching registry records")
	records, err := dnsimpleProvider.AllRegistryRecords()
	if err != nil {
		fmt.Printf("Cannot fetch registry records: %s\n", err)
		os.Exit(1)
	} else {
		for zone, zoneRecords := range records {
			fmt.Printf("Records for: %s\n", zone.Name)
			for _, record := range zoneRecords {
				fmt.Println(record.Info())
			}
		}
	}
}
