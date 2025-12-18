package source

import (
	"context"
	"github.com/matic-insurance/dns-tager/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istionetworking "istio.io/api/networking/v1alpha3"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istiofake "istio.io/client-go/pkg/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestIstioVirtualService(t *testing.T) {
	t.Parallel()

	t.Run("Endpoints", testVirtualServiceEndpoints)
}

func testVirtualServiceEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		targetNamespace   string
		gwConfigs         []fakeGatewayConfig
		vsConfigs         []fakeVirtualServiceConfig
		expectedEndpoints []*registry.Endpoint
		expectError       bool
	}{
		{
			name: "No Services",
		},
		{
			name: "Internal Virtual Service with mesh gateway",
			vsConfigs: []fakeVirtualServiceConfig{
				{
					name:      "webserver",
					namespace: namespace,
					gateways:  []string{"mesh"},
					dnsnames:  []string{"webserver.testing.svc.local"},
				},
			},
		},
		{
			name: "Virtual Service with another controller",
			vsConfigs: []fakeVirtualServiceConfig{
				{
					name:      "webserver",
					namespace: namespace,
					gateways:  []string{"mesh", "webserver-gateway"},
					dnsnames:  []string{"webserver.testing.svc.local", "webserver.dummy.host"},
					annotations: map[string]string{
						controllerAnnotationKey: "ignored",
					},
				},
			},
			gwConfigs: []fakeGatewayConfig{
				{
					name:      "webserver-gateway",
					namespace: namespace,
					dnsnames:  []string{"webserver.dummy.host"},
				},
			},
		},
		{
			name: "Virtual Service with internal and dedicated gateway",
			vsConfigs: []fakeVirtualServiceConfig{
				{
					name:      "webserver",
					namespace: namespace,
					gateways:  []string{"mesh", "webserver-gateway"},
					dnsnames:  []string{"webserver.testing.svc.local", "webserver.dummy.host"},
				},
			},
			gwConfigs: []fakeGatewayConfig{
				{
					name:      "webserver-gateway",
					namespace: namespace,
					dnsnames:  []string{"webserver.dummy.host"},
				},
			},
			expectedEndpoints: []*registry.Endpoint{
				{
					Host:     "webserver.dummy.host",
					Resource: "virtualservice/testing/webserver",
				},
			},
		},
		{
			name: "Virtual Service with internal and wildcard gateway",
			vsConfigs: []fakeVirtualServiceConfig{
				{
					name:      "webserver",
					namespace: namespace,
					gateways:  []string{"mesh", "ingress/webserver-gateway"},
					dnsnames:  []string{"webserver.testing.svc.local", "webserver.dummy.host"},
				},
			},
			gwConfigs: []fakeGatewayConfig{
				{
					name:      "webserver-gateway",
					namespace: "ingress",
					dnsnames:  []string{"*.dummy.host"},
				},
			},
			expectedEndpoints: []*registry.Endpoint{
				{
					Host:     "webserver.dummy.host",
					Resource: "virtualservice/testing/webserver",
				},
			},
		},
		{
			name: "Virtual Service with annotations and wildcard gateway",
			vsConfigs: []fakeVirtualServiceConfig{
				{
					name:      "webserver",
					namespace: namespace,
					gateways:  []string{"mesh", "ingress/webserver-gateway"},
					dnsnames:  []string{"webserver.dummy.host"},
					annotations: map[string]string{
						hostnameAnnotationKey: "webserver1.dummy.host,webserver2.dummy.host",
					},
				},
			},
			gwConfigs: []fakeGatewayConfig{
				{
					name:      "webserver-gateway",
					namespace: "ingress",
					dnsnames:  []string{"*.dummy.host"},
				},
			},
			expectedEndpoints: []*registry.Endpoint{
				{
					Host:     "webserver.dummy.host",
					Resource: "virtualservice/testing/webserver",
				},
				{
					Host:     "webserver1.dummy.host",
					Resource: "virtualservice/testing/webserver",
				},
				{
					Host:     "webserver2.dummy.host",
					Resource: "virtualservice/testing/webserver",
				},
			},
		},
		{
			name: "Multiple services with single gateway",
			vsConfigs: []fakeVirtualServiceConfig{
				{
					name:      "fake1",
					namespace: namespace,
					gateways:  []string{"mesh", "ingress/webserver-gateway"},
					dnsnames:  []string{"fake1.dummy.host"},
				},
				{
					name:      "fake2",
					namespace: namespace,
					gateways:  []string{"mesh", "ingress/webserver-gateway"},
					dnsnames:  []string{"fake2.dummy.host"},
				},
			},
			gwConfigs: []fakeGatewayConfig{
				{
					name:      "webserver-gateway",
					namespace: "ingress",
					dnsnames:  []string{"*.dummy.host"},
				},
			},
			expectedEndpoints: []*registry.Endpoint{
				{
					Host:     "fake1.dummy.host",
					Resource: "virtualservice/testing/fake1",
				},
				{
					Host:     "fake2.dummy.host",
					Resource: "virtualservice/testing/fake2",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeKubernetesClient := fake.NewSimpleClientset()
			fakeIstioClient := istiofake.NewSimpleClientset()

			for _, gatewayItem := range tt.gwConfigs {
				gateway := gatewayItem.Config()
				_, err := fakeIstioClient.NetworkingV1alpha3().Gateways(gateway.Namespace).Create(context.Background(), gateway, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			for _, vsItem := range tt.vsConfigs {
				virtualService := vsItem.Config()
				_, err := fakeIstioClient.NetworkingV1alpha3().VirtualServices(virtualService.Namespace).Create(context.Background(), virtualService, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			virtualServiceSource, err := NewIstioVirtualServiceSource(context.TODO(), fakeKubernetesClient, fakeIstioClient, tt.targetNamespace, nil)
			require.NoError(t, err)

			res, err := virtualServiceSource.Endpoints(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			validateEndpoints(t, res, tt.expectedEndpoints)
		})
	}
}

type fakeVirtualServiceConfig struct {
	name        string
	namespace   string
	annotations map[string]string
	gateways    []string
	dnsnames    []string
}

func (c fakeVirtualServiceConfig) Config() *networkingv1alpha3.VirtualService {
	vs := istionetworking.VirtualService{
		Gateways: c.gateways,
		Hosts:    c.dnsnames,
	}

	return &networkingv1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.name,
			Namespace:   c.namespace,
			Annotations: c.annotations,
		},
		Spec: *vs.DeepCopy(),
	}
}

type fakeGatewayConfig struct {
	name        string
	namespace   string
	annotations map[string]string
	dnsnames    []string
}

func (c fakeGatewayConfig) Config() *networkingv1alpha3.Gateway {
	gw := &networkingv1alpha3.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.name,
			Namespace:   c.namespace,
			Annotations: c.annotations,
		},
		Spec: istionetworking.Gateway{
			Servers:  nil,
			Selector: nil,
		},
	}

	gw.Spec.Servers = []*istionetworking.Server{
		{Hosts: c.dnsnames},
	}

	return gw
}
