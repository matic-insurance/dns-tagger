package cloudflare

import (
	"context"
	"testing"

	cf "github.com/cloudflare/cloudflare-go"
	"github.com/matic-insurance/dns-tager/pkg"
	"github.com/matic-insurance/dns-tager/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	testProvider *cloudflareProvider
	testApi      *mockCloudflareDNSApi
	zone         = &registry.Zone{Name: "dummy.host"}
)

type mockCloudflareDNSApi struct {
	mock.Mock
}

func newTestProvider() *cloudflareProvider {
	testApi = &mockCloudflareDNSApi{}
	return &cloudflareProvider{
		api:     testApi,
		zones:   []string{zone.Name},
		zoneIDs: map[string]string{zone.Name: "zone-123"},
		cfg:     &pkg.Config{Apply: true},
	}
}

func TestCloudflareProvider(t *testing.T) {
	t.Run("UpdateRegistryRecord_Success", testUpdateRegistryRecord_Success)
	t.Run("UpdateRegistryRecord_NoApply", testUpdateRegistryRecord_NoApply)
	t.Run("ReadZones_SplitsAndLinks", testReadZones_SplitsAndLinks)
}

func testUpdateRegistryRecord_Success(t *testing.T) {
	testProvider = newTestProvider()
	record := &registry.Record{Name: "webserver.dummy.host", Owner: "cluster-1", Resource: "ingress/test/webserver"}

	cfRecords := []cf.DNSRecord{{ID: "rec-234", Name: "webserver.dummy.host", Type: "TXT"}}
	testApi.On("ListDNSRecords", context.Background(), cf.ZoneIdentifier("zone-123"), mock.Anything).Return(cfRecords, &cf.ResultInfo{}, nil)
	testApi.On("UpdateDNSRecord", context.Background(), cf.ZoneIdentifier("zone-123"), cf.UpdateDNSRecordParams{ID: "rec-234", Content: record.Info()}).Return(cf.DNSRecord{}, nil)

	updates, err := testProvider.UpdateRegistryRecord(context.Background(), zone, record)

	assert.NoError(t, err)
	assert.Equal(t, 1, updates, "Correct updates count returned")
	testApi.AssertExpectations(t)
}

func testUpdateRegistryRecord_NoApply(t *testing.T) {
	testProvider = newTestProvider()
	testProvider.cfg.Apply = false
	record := &registry.Record{Name: "webserver.dummy.host", Owner: "cluster-1", Resource: "ingress/test/webserver"}

	updates, err := testProvider.UpdateRegistryRecord(context.Background(), zone, record)

	assert.NoError(t, err)
	assert.Equal(t, 1, updates, "Correct updates count returned")
	testApi.AssertNotCalled(t, "UpdateDNSRecord")
}

func testReadZones_SplitsAndLinks(t *testing.T) {
	testProvider = newTestProvider()
	records := []cf.DNSRecord{
		{Name: "webserver.dummy.host", Type: "A", Content: "1.2.3.4"},
		{Name: "edns-webserver.dummy.host", Type: "TXT", Content: "heritage=external-dns,external-dns/owner=cluster-1,external-dns/resource=ingress/test/webserver"},
		{Name: "unmanaged.dummy.host", Type: "A", Content: "5.6.7.8"},
		{Name: "noise.dummy.host", Type: "TXT", Content: "some-other-txt"},
	}
	testApi.On("ListDNSRecords", context.Background(), cf.ZoneIdentifier("zone-123"), cf.ListDNSRecordsParams{}).Return(records, &cf.ResultInfo{}, nil)

	zones, err := testProvider.ReadZones(context.Background())

	assert.NoError(t, err)
	assert.Len(t, zones, 1)
	hosts := zones[0].Hosts
	assert.Len(t, hosts, 2, "only A records become hosts")

	var webserver, unmanaged *registry.Host
	for _, h := range hosts {
		switch h.Name {
		case "webserver.dummy.host":
			webserver = h
		case "unmanaged.dummy.host":
			unmanaged = h
		}
	}
	assert.NotNil(t, webserver)
	assert.True(t, webserver.IsManaged(), "host with matching edns- TXT record is managed")
	assert.Equal(t, "cluster-1", webserver.RegistryRecords[0].Owner)
	assert.NotNil(t, unmanaged)
	assert.False(t, unmanaged.IsManaged(), "host without registry record is unmanaged")
}

func (_m *mockCloudflareDNSApi) ZoneIDByName(zoneName string) (string, error) {
	args := _m.Called(zoneName)
	return args.String(0), args.Error(1)
}

func (_m *mockCloudflareDNSApi) ListDNSRecords(ctx context.Context, rc *cf.ResourceContainer, params cf.ListDNSRecordsParams) ([]cf.DNSRecord, *cf.ResultInfo, error) {
	args := _m.Called(ctx, rc, params)
	var r0 []cf.DNSRecord
	if args.Get(0) != nil {
		r0 = args.Get(0).([]cf.DNSRecord)
	}
	var r1 *cf.ResultInfo
	if args.Get(1) != nil {
		r1 = args.Get(1).(*cf.ResultInfo)
	}
	return r0, r1, args.Error(2)
}

func (_m *mockCloudflareDNSApi) UpdateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.UpdateDNSRecordParams) (cf.DNSRecord, error) {
	args := _m.Called(ctx, rc, params)
	var r0 cf.DNSRecord
	if args.Get(0) != nil {
		r0 = args.Get(0).(cf.DNSRecord)
	}
	return r0, args.Error(1)
}
