package source

import (
	"context"
	"github.com/matic-insurance/dns-tager/registry"
	"github.com/stretchr/testify/require"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestIngress(t *testing.T) {
	t.Run("TestIngress_Endpoints", testIngressEndpoints)
}

func testIngressEndpoints(t *testing.T) {
	namespace := "testing"
	tests := []struct {
		name              string
		ingressItems      []fakeIngress
		expectedEndpoints []*registry.Endpoint
	}{
		{
			name: "No Items",
		},
		{
			name: "Different controller spec",
			ingressItems: []fakeIngress{
				{
					name:      "fake1",
					namespace: namespace,
					dnsnames:  []string{"fake1.dummy.host"},
					annotations: map[string]string{
						controllerAnnotationKey: "another-controller",
					},
				},
			},
		},
		{
			name: "Two host ingresses",
			ingressItems: []fakeIngress{
				{
					name:      "fake1",
					namespace: namespace,
					dnsnames:  []string{"fake1.dummy.host"},
				},
				{
					name:      "fake2",
					namespace: namespace,
					dnsnames:  []string{"fake2.dummy.host"},
				},
			},
			expectedEndpoints: []*registry.Endpoint{
				{
					Host:     "fake1.dummy.host",
					Resource: "ingress/testing/fake1",
				},
				{
					Host:     "fake2.dummy.host",
					Resource: "ingress/testing/fake2",
				},
			},
		},
		{
			name: "Hosts in annotations",
			ingressItems: []fakeIngress{
				{
					name:      "fake1",
					namespace: namespace,
					dnsnames:  []string{"fake1.dummy.host"},
					annotations: map[string]string{
						hostnameAnnotationKey: "fake2.dummy.host,fake3.dummy.host",
					},
				},
			},
			expectedEndpoints: []*registry.Endpoint{
				{
					Host:     "fake1.dummy.host",
					Resource: "ingress/testing/fake1",
				},
				{
					Host:     "fake2.dummy.host",
					Resource: "ingress/testing/fake1",
				},
				{
					Host:     "fake3.dummy.host",
					Resource: "ingress/testing/fake1",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			t.Parallel()

			fakeClient := fake.NewSimpleClientset()
			for _, item := range tt.ingressItems {
				ingress := item.Ingress()
				_, err := fakeClient.NetworkingV1().Ingresses(ingress.Namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			source, _ := NewIngressSource(
				context.TODO(),
				fakeClient,
				namespace,
			)

			// Compare retrieved ingresses with expected ones
			res, _ := source.Endpoints(context.Background())
			validateEndpoints(t, res, tt.expectedEndpoints)
		})
	}
}

type fakeIngress struct {
	dnsnames    []string
	tlsdnsnames [][]string
	namespace   string
	name        string
	annotations map[string]string
	labels      map[string]string
}

func (ing fakeIngress) Ingress() *networkv1.Ingress {
	ingress := &networkv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ing.namespace,
			Name:        ing.name,
			Annotations: ing.annotations,
			Labels:      ing.labels,
		},
		Spec: networkv1.IngressSpec{
			Rules: []networkv1.IngressRule{},
		},
		Status: networkv1.IngressStatus{
			LoadBalancer: networkv1.IngressLoadBalancerStatus{
				Ingress: []networkv1.IngressLoadBalancerIngress{},
			},
		},
	}
	for _, dnsname := range ing.dnsnames {
		ingress.Spec.Rules = append(ingress.Spec.Rules, networkv1.IngressRule{
			Host: dnsname,
		})
	}
	for _, hosts := range ing.tlsdnsnames {
		ingress.Spec.TLS = append(ingress.Spec.TLS, networkv1.IngressTLS{
			Hosts: hosts,
		})
	}
	return ingress
}
