package dnsimple

import (
	"context"
	"fmt"
	"github.com/dnsimple/dnsimple-go/dnsimple"
	"github.com/matic-insurance/external-dns-dialer/provider"
	"golang.org/x/oauth2"
	"os"
	"strconv"
)

type dnsimpleProvider struct {
	provider.BaseProvider
	client    *dnsimple.ZonesService
	identity  *dnsimple.IdentityService
	accountID interface{}
}

func (p dnsimpleProvider) Whoami() string {
	return fmt.Sprintf("DNSimple for Account %s", p.accountID)
}

func NewDnsimpleProvider() (provider.Provider, error) {
	oauthToken := os.Getenv("DNSIMPLE_OAUTH")
	if len(oauthToken) == 0 {
		return nil, fmt.Errorf("no dnsimple authentication provided provided (DNSIMPLE_OAUTH or DNSIMPLE_ACCOUNT/DNSIMPLE_TOKEN are missing)")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: oauthToken})
	tc := oauth2.NewClient(context.Background(), ts)

	client := dnsimple.NewClient(tc)
	client.SetUserAgent(fmt.Sprintf("Kubernetes ExternalDNS Dialer"))

	providerInstance := &dnsimpleProvider{
		client:   client.Zones,
		identity: client.Identity,
	}

	whoamiResponse, err := providerInstance.identity.Whoami(context.Background())
	if err != nil {
		return nil, err
	}
	providerInstance.accountID = int64ToString(whoamiResponse.Data.Account.ID)
	return providerInstance, nil
}

func int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}
