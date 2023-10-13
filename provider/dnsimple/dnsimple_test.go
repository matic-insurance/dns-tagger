package dnsimple

import (
	"context"
	"github.com/dnsimple/dnsimple-go/dnsimple"
	"github.com/matic-insurance/external-dns-dialer/pkg"
	"github.com/matic-insurance/external-dns-dialer/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

var (
	testProvider dnsimpleProvider
	testApi      *mockDnsimpleZoneServiceInterface
	zone         = &registry.Zone{Name: "dummy.host"}
)

type mockDnsimpleZoneServiceInterface struct {
	mock.Mock
}

func TestDnsimpleProvider(t *testing.T) {
	testApi = &mockDnsimpleZoneServiceInterface{}
	testProvider = dnsimpleProvider{client: testApi, accountID: "123", cfg: &pkg.Config{}}

	t.Run("UpdateRegistryRecord_Success", testDnsimpleProviderUpdateRegistryRecord_Success)
	t.Run("UpdateRegistryRecord_DryRun", testDnsimpleProviderUpdateRegistryRecord_DryRun)
}

func testDnsimpleProviderUpdateRegistryRecord_DryRun(t *testing.T) {
	testProvider.cfg.DryRun = true
	record := &registry.Record{Name: "webserver.dummy.host", Owner: "cluster-1", Resource: "ingress/test/webserver"}
	updates, err := testProvider.UpdateRegistryRecord(context.Background(), zone, record)

	assert.Equal(t, 1, updates, "Correct updates count returned")
	assert.NoError(t, err)
}

func testDnsimpleProviderUpdateRegistryRecord_Success(t *testing.T) {
	record := &registry.Record{Name: "webserver.dummy.host", Owner: "cluster-1", Resource: "ingress/test/webserver"}

	dnsimpleRecords := []dnsimple.ZoneRecord{{ID: 234, Name: "webserver.dummy.host"}}
	testApi.On("ListRecords", context.Background(), "123", zone.Name, mock.Anything).Return(dnsimpleZoneResponse(dnsimpleRecords), nil)
	testApi.On("UpdateRecord", context.Background(), "123", zone.Name, dnsimpleRecords[0].ID, dnsimple.ZoneRecordAttributes{Content: record.Info()}).Return(&dnsimple.ZoneRecordResponse{}, nil)

	updates, err := testProvider.UpdateRegistryRecord(context.Background(), zone, record)

	assert.NoError(t, err)
	assert.Equal(t, 1, updates, "Correct updates count returned")
}

func dnsimpleZoneResponse(records []dnsimple.ZoneRecord) *dnsimple.ZoneRecordsResponse {
	return &dnsimple.ZoneRecordsResponse{Data: records, Response: dnsimple.Response{Pagination: &dnsimple.Pagination{}}}
}

func (_m *mockDnsimpleZoneServiceInterface) ListRecords(ctx context.Context, accountID string, zoneID string, options *dnsimple.ZoneRecordListOptions) (*dnsimple.ZoneRecordsResponse, error) {
	args := _m.Called(ctx, accountID, zoneID, options)
	var r0 *dnsimple.ZoneRecordsResponse

	if args.Get(0) != nil {
		r0 = args.Get(0).(*dnsimple.ZoneRecordsResponse)
	}

	return r0, args.Error(1)
}

func (_m *mockDnsimpleZoneServiceInterface) ListZones(ctx context.Context, accountID string, options *dnsimple.ZoneListOptions) (*dnsimple.ZonesResponse, error) {
	args := _m.Called(ctx, accountID, options)
	var r0 *dnsimple.ZonesResponse

	if args.Get(0) != nil {
		r0 = args.Get(0).(*dnsimple.ZonesResponse)
	}

	return r0, args.Error(1)
}

func (_m *mockDnsimpleZoneServiceInterface) UpdateRecord(ctx context.Context, accountID string, zoneID string, recordID int64, recordAttributes dnsimple.ZoneRecordAttributes) (*dnsimple.ZoneRecordResponse, error) {
	args := _m.Called(ctx, accountID, zoneID, recordID, recordAttributes)
	var r0 *dnsimple.ZoneRecordResponse

	if args.Get(0) != nil {
		r0 = args.Get(0).(*dnsimple.ZoneRecordResponse)
	}

	return r0, args.Error(1)
}
