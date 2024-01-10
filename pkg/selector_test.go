package pkg

import (
	"context"
	"errors"
	"github.com/matic-insurance/dns-tager/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

var (
	testProvider          *mockProvider
	currentOwnerId        = "cluster-2"
	testEndpointResource  = "ingress/test/webserver"
	testEndpointResource2 = "ingress/test/webserver2"
	testEndpointHost      = "webserver.dummy.host"
	cfg                   = &Config{
		CurrentOwnerID:   currentOwnerId,
		PreviousOwnerIDs: []string{"cluster-1"},
	}
)

type mockProvider struct {
	mock.Mock
}

func (p *mockProvider) ReadZones(_ context.Context) ([]*registry.Zone, error) {
	panic("implement me")
}

func (p *mockProvider) UpdateRegistryRecord(ctx context.Context, zone *registry.Zone, record *registry.Record) (updatedRecords int, err error) {
	args := p.Called(ctx, zone, record)
	return args.Int(0), args.Error(1)
}

func (p *mockProvider) Whoami(_ context.Context) string {
	panic("implement me")
}

func TestSelector_UpdateRegistryRecords_NoEndpointHost(t *testing.T) {
	testProvider = &mockProvider{}
	selector := Selector{provider: testProvider, cfg: cfg}
	endpoint := &registry.Endpoint{Host: "another.dummy.host", Resource: testEndpointResource}
	zone := createTestZone(cfg.PreviousOwnerIDs[0], "ingress/test/webserver")

	updates, err := selector.claimEndpoint(context.Background(), endpoint, zone)
	testProvider.AssertNotCalled(t, "UpdateRegistryRecord")
	assert.Equal(t, 0, updates, "Zero updates count returned")
	assert.NoError(t, err)
}

func TestSelector_UpdateRegistryRecords_SameOwner(t *testing.T) {
	testProvider = &mockProvider{}
	selector := Selector{provider: testProvider, cfg: cfg}
	endpoint := &registry.Endpoint{Host: testEndpointHost, Resource: testEndpointResource}
	zone := createTestZone(currentOwnerId, testEndpointResource)

	updates, err := selector.claimEndpoint(context.Background(), endpoint, zone)
	testProvider.AssertNotCalled(t, "UpdateRegistryRecord")
	assert.Equal(t, 0, updates, "Zero updates count returned")
	assert.NoError(t, err)
}

func TestSelector_UpdateRegistryRecords_NotAllowedOwner(t *testing.T) {
	testProvider = &mockProvider{}
	selector := Selector{provider: testProvider, cfg: cfg}
	endpoint := &registry.Endpoint{Host: testEndpointHost, Resource: testEndpointResource}
	zone := createTestZone("cluster-0", testEndpointResource)

	updates, err := selector.claimEndpoint(context.Background(), endpoint, zone)
	testProvider.AssertNotCalled(t, "UpdateRegistryRecord")
	assert.Equal(t, 0, updates, "Zero updates count returned")
	assert.NoError(t, err)
}

func TestSelector_UpdateRegistryRecords_ProviderError(t *testing.T) {
	testProvider = &mockProvider{}
	selector := Selector{provider: testProvider, cfg: cfg}
	endpoint := &registry.Endpoint{Host: testEndpointHost, Resource: testEndpointResource}
	zone := createTestZone(cfg.PreviousOwnerIDs[0], testEndpointResource)

	testProvider.On("UpdateRegistryRecord", context.Background(), mock.Anything, mock.Anything).Return(1, nil).Once()
	testProvider.On("UpdateRegistryRecord", context.Background(), mock.Anything, mock.Anything).Return(0, errors.New("test"))

	updates, err := selector.claimEndpoint(context.Background(), endpoint, zone)
	assert.Equal(t, 1, updates, "Made updates count returned")
	assert.Error(t, err)
}

func TestSelector_UpdateRegistryRecords_MultipleRegistries(t *testing.T) {
	testProvider := &mockProvider{}
	selector := Selector{provider: testProvider, cfg: cfg}
	endpoint := &registry.Endpoint{Host: testEndpointHost, Resource: testEndpointResource}
	zone := createTestZone(cfg.PreviousOwnerIDs[0], "")

	testProvider.On("UpdateRegistryRecord", context.Background(), zone, mock.MatchedBy(func(record *registry.Record) bool {
		return record.Owner == currentOwnerId && record.Resource == testEndpointResource && record.Name == "registry1-"+testEndpointHost
	})).Return(1, nil)
	testProvider.On("UpdateRegistryRecord", context.Background(), zone, mock.MatchedBy(func(record *registry.Record) bool {
		return record.Owner == currentOwnerId && record.Resource == testEndpointResource && record.Name == "registry1-cname-"+testEndpointHost
	})).Return(1, nil)

	updates, err := selector.claimEndpoint(context.Background(), endpoint, zone)
	testProvider.AssertNumberOfCalls(t, "UpdateRegistryRecord", 2)
	assert.Equal(t, 2, updates, "Correct updates count returned")
	assert.NoError(t, err)
}

func TestSelector_UpdateRegistryRecords_SameResource(t *testing.T) {
	testProvider = &mockProvider{}
	selector := Selector{provider: testProvider, cfg: cfg}
	endpoint := &registry.Endpoint{Host: testEndpointHost, Resource: testEndpointResource}
	zone := createTestZone(currentOwnerId, testEndpointResource)

	updates, err := selector.claimEndpointResource(context.Background(), endpoint, zone)
	testProvider.AssertNotCalled(t, "UpdateRegistryRecord")
	assert.Equal(t, 0, updates, "Zero updates count returned")
	assert.NoError(t, err)
}

func TestSelector_UpdateRegistryRecords_ProviderErrorResource(t *testing.T) {
	testProvider = &mockProvider{}
	selector := Selector{provider: testProvider, cfg: cfg}
	endpoint := &registry.Endpoint{Host: testEndpointHost, Resource: testEndpointResource2}
	zone := createTestZone(currentOwnerId, testEndpointResource)

	testProvider.On("UpdateRegistryRecord", context.Background(), mock.Anything, mock.Anything).Return(1, nil).Once()
	testProvider.On("UpdateRegistryRecord", context.Background(), mock.Anything, mock.Anything).Return(0, errors.New("test"))

	updates, err := selector.claimEndpointResource(context.Background(), endpoint, zone)
	assert.Equal(t, 1, updates, "Made updates count returned")
	assert.Error(t, err)
}

func createTestZone(owner string, resource string) *registry.Zone {
	zone := registry.NewZone("dummy.host")
	host := registry.NewHost(testEndpointHost, "", "")
	host.AddRegistryRecord(&registry.Record{Name: "registry1-" + testEndpointHost, Owner: owner, Resource: resource})
	host.AddRegistryRecord(&registry.Record{Name: "registry1-cname-" + testEndpointHost, Owner: owner, Resource: resource})
	zone.AddHost(host)
	return zone
}
