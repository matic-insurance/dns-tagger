package main

import (
	"fmt"
	"github.com/matic-insurance/external-dns-dialer/provider/dnsimple"
	"os"
)

func main() {
	dnsimpleProvider, err := dnsimple.NewDnsimpleProvider()
	if err != nil {
		fmt.Printf("Cannot init dnSimple: %s", err)
		os.Exit(1)
	}
	fmt.Printf("%s", dnsimpleProvider.Whoami())
}
