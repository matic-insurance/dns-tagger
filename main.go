package main

import (
	"fmt"
	"github.com/matic-insurance/external-dns-dialer/provider/dnsimple"
	"github.com/matic-insurance/external-dns-dialer/registry"
	"os"
)

func main() {
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
