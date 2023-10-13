/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package source

import (
	"context"
	"fmt"
	"github.com/matic-insurance/external-dns-dialer/registry"
	log "github.com/sirupsen/logrus"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	netinformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	// ALBDualstackAnnotationKey is the annotation used for determining if an ALB ingress is dualstack
	ALBDualstackAnnotationKey = "alb.ingress.kubernetes.io/ip-address-type"
	// ALBDualstackAnnotationValue is the value of the ALB dualstack annotation that indicates it is dualstack
	ALBDualstackAnnotationValue = "dualstack"

	// Possible values for the ingress-hostname-source annotation
	IngressHostnameSourceAnnotationOnlyValue   = "annotation-only"
	IngressHostnameSourceDefinedHostsOnlyValue = "defined-hosts-only"

	IngressClassAnnotationKey = "kubernetes.io/ingress.class"
)

type ingressSource struct {
	client          kubernetes.Interface
	namespace       string
	ingressInformer netinformers.IngressInformer
}

// NewIngressSource creates a new ingressSource with the given config.
func NewIngressSource(ctx context.Context, kubeClient kubernetes.Interface, namespace string) (Source, error) {
	// Use shared informer to listen for add/update/delete of ingresses in the specified namespace.
	// Set resync period to 0, to prevent processing when nothing has changed.
	informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace))
	ingressInformer := informerFactory.Networking().V1().Ingresses()

	// Add default resource event handlers to properly initialize informer.
	ingressInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
			},
		},
	)

	informerFactory.Start(ctx.Done())

	// wait for the local cache to be populated.
	if err := waitForCacheSync(context.Background(), informerFactory); err != nil {
		return nil, err
	}

	sc := &ingressSource{
		client:          kubeClient,
		namespace:       namespace,
		ingressInformer: ingressInformer,
	}
	return sc, nil
}

// Endpoints returns endpoint objects for each host-target combination that should be processed.
// Retrieves all ingress resources on all namespaces
func (sc *ingressSource) Endpoints(ctx context.Context) ([]*registry.Endpoint, error) {
	ingresses, err := sc.ingressInformer.Lister().Ingresses(sc.namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var endpoints []*registry.Endpoint

	for _, ing := range ingresses {
		// Check controller annotation to see if we are responsible.
		controller, ok := ing.Annotations[controllerAnnotationKey]
		if ok && controller != controllerAnnotationValue {
			log.Debugf("Skipping ingress %s/%s because controller value does not match, found: %s, required: %s",
				ing.Namespace, ing.Name, controller, controllerAnnotationValue)
			continue
		}

		ingEndpoints := endpointsFromIngress(ing)

		if len(ingEndpoints) == 0 {
			log.Debugf("No endpoints could be generated from ingress %s/%s", ing.Namespace, ing.Name)
			continue
		}

		log.Debugf("Endpoints generated from ingress: %s/%s: %v", ing.Namespace, ing.Name, ingEndpoints)
		endpoints = append(endpoints, ingEndpoints...)
	}

	return endpoints, nil
}

// endpointsFromIngress extracts the endpoints from ingress object
func endpointsFromIngress(ing *networkv1.Ingress) []*registry.Endpoint {
	resource := fmt.Sprintf("ingress/%s/%s", ing.Namespace, ing.Name)

	targets := targetsFromIngressStatus(ing.Status)

	var definedHostsEndpoints []*registry.Endpoint
	// Gather endpoints defined on hosts sections of the ingress
	for _, rule := range ing.Spec.Rules {
		if rule.Host == "" {
			continue
		}
		definedHostsEndpoints = append(definedHostsEndpoints, registry.NewEndpoint(rule.Host, resource, targets))
	}

	// Gather endpoints defined on annotations in the ingress
	var annotationEndpoints []*registry.Endpoint
	for _, hostname := range getHostnamesFromAnnotations(ing.Annotations) {
		annotationEndpoints = append(annotationEndpoints, registry.NewEndpoint(hostname, resource, targets))
	}

	// Include endpoints according to the hostname source annotation in our final list
	var endpoints []*registry.Endpoint
	endpoints = append(endpoints, definedHostsEndpoints...)
	endpoints = append(endpoints, annotationEndpoints...)

	return endpoints
}

func targetsFromIngressStatus(status networkv1.IngressStatus) []string {
	var targets []string

	for _, lb := range status.LoadBalancer.Ingress {
		if lb.IP != "" {
			targets = append(targets, lb.IP)
		}
		if lb.Hostname != "" {
			targets = append(targets, lb.Hostname)
		}
	}

	return targets
}

func (sc *ingressSource) AddEventHandler(ctx context.Context, handler func()) {
	log.Debug("Adding event handler for ingress")

	// Right now there is no way to remove event handler from informer, see:
	// https://github.com/kubernetes/kubernetes/issues/79610
	sc.ingressInformer.Informer().AddEventHandler(eventHandlerFunc(handler))
}
