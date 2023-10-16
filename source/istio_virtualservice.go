package source

import (
	"context"
	"fmt"
	"github.com/matic-insurance/dns-tager/registry"
	log "github.com/sirupsen/logrus"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	istioinformers "istio.io/client-go/pkg/informers/externalversions"
	networkingv1alpha3informer "istio.io/client-go/pkg/informers/externalversions/networking/v1alpha3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"strings"
)

// IstioMeshGateway is the built in gateway for all sidecars
const IstioMeshGateway = "mesh"

// virtualServiceSource is an implementation of Source for Istio VirtualService objects.
// The implementation uses the spec.hosts values for the hostnames.
// Use targetAnnotationKey to explicitly set Endpoint.
type virtualServiceSource struct {
	kubeClient             kubernetes.Interface
	istioClient            istioclient.Interface
	namespace              string
	serviceInformer        coreinformers.ServiceInformer
	virtualserviceInformer networkingv1alpha3informer.VirtualServiceInformer
}

// NewIstioVirtualServiceSource creates a new virtualServiceSource with the given config.
func NewIstioVirtualServiceSource(ctx context.Context, kubeClient kubernetes.Interface, istioClient istioclient.Interface, namespace string) (Source, error) {
	// Use shared informers to listen for add/update/delete of services/pods/nodes in the specified namespace.
	// Set resync period to 0, to prevent processing when nothing has changed
	informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace))
	serviceInformer := informerFactory.Core().V1().Services()
	istioInformerFactory := istioinformers.NewSharedInformerFactoryWithOptions(istioClient, 0, istioinformers.WithNamespace(namespace))
	virtualServiceInformer := istioInformerFactory.Networking().V1alpha3().VirtualServices()

	// Add default resource event handlers to properly initialize informer.
	_, err := serviceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Debug("service added")
			},
		},
	)
	if err != nil {
		return nil, err
	}

	_, err = virtualServiceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Debug("virtual service added")
			},
		},
	)
	if err != nil {
		return nil, err
	}

	informerFactory.Start(ctx.Done())
	istioInformerFactory.Start(ctx.Done())

	// wait for the local cache to be populated.
	if err := waitForCacheSync(context.Background(), informerFactory); err != nil {
		return nil, err
	}
	if err := waitForCacheSync(context.Background(), istioInformerFactory); err != nil {
		return nil, err
	}

	return &virtualServiceSource{
		kubeClient:             kubeClient,
		istioClient:            istioClient,
		namespace:              namespace,
		serviceInformer:        serviceInformer,
		virtualserviceInformer: virtualServiceInformer,
	}, nil
}

// Endpoints returns endpoint objects for each host-target combination that should be processed.
// Retrieves all VirtualService resources in the source's namespace(s).
func (sc *virtualServiceSource) Endpoints(ctx context.Context) ([]*registry.Endpoint, error) {
	virtualServices, err := sc.virtualserviceInformer.Lister().VirtualServices(sc.namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var endpoints []*registry.Endpoint

	for _, virtualService := range virtualServices {
		// Check controller annotation to see if we are responsible.
		controller, ok := virtualService.Annotations[controllerAnnotationKey]
		if ok && controller != controllerAnnotationValue {
			log.Debugf("Skipping VirtualService %s/%s because controller value does not match, found: %s, required: %s",
				virtualService.Namespace, virtualService.Name, controller, controllerAnnotationValue)
			continue
		}

		vsEndpoints, err := sc.endpointsFromVirtualService(ctx, virtualService)
		if err != nil {
			return nil, err
		}

		if len(vsEndpoints) == 0 {
			log.Debugf("No endpoints could be generated from VirtualService %s/%s", virtualService.Namespace, virtualService.Name)
			continue
		} else {
			log.Debugf("Endpoints generated from VirtualService: %s/%s: %v", virtualService.Namespace, virtualService.Name, vsEndpoints)
			endpoints = append(endpoints, vsEndpoints...)
		}
	}

	return endpoints, nil
}

// AddEventHandler adds an event handler that should be triggered if the watched Istio VirtualService changes.
func (sc *virtualServiceSource) AddEventHandler(_ context.Context, handler func()) {
	log.Debug("Adding event handler for Istio VirtualService")

	sc.virtualserviceInformer.Informer().AddEventHandler(eventHandlerFunc(handler))
}

func (sc *virtualServiceSource) getGateway(ctx context.Context, gatewayStr string, virtualService *networkingv1alpha3.VirtualService) (*networkingv1alpha3.Gateway, error) {
	if gatewayStr == "" || gatewayStr == IstioMeshGateway {
		// This refers to "all sidecars in the mesh"; ignore.
		return nil, nil
	}

	namespace, name, err := parseGateway(gatewayStr)
	if err != nil {
		log.Debugf("Failed parsing gatewayStr %s of VirtualService %s/%s", gatewayStr, virtualService.Namespace, virtualService.Name)
		return nil, err
	}
	if namespace == "" {
		namespace = virtualService.Namespace
	}

	gateway, err := sc.istioClient.NetworkingV1alpha3().Gateways(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		log.Warnf("VirtualService (%s/%s) references non-existent gateway: %s ", virtualService.Namespace, virtualService.Name, gatewayStr)
		return nil, nil
	} else if err != nil {
		log.Errorf("Failed retrieving gateway %s referenced by VirtualService %s/%s: %v", gatewayStr, virtualService.Namespace, virtualService.Name, err)
		return nil, err
	}
	if gateway == nil {
		log.Debugf("Gateway %s referenced by VirtualService %s/%s not found: %v", gatewayStr, virtualService.Namespace, virtualService.Name, err)
		return nil, nil
	}
	return gateway, nil
}

// endpointsFromVirtualService extracts the endpoints from an Istio VirtualService Config object
func (sc *virtualServiceSource) endpointsFromVirtualService(ctx context.Context, virtualservice *networkingv1alpha3.VirtualService) ([]*registry.Endpoint, error) {
	var endpoints []*registry.Endpoint
	var hosts []string

	resource := fmt.Sprintf("virtualservice/%s/%s", virtualservice.Namespace, virtualservice.Name)

	for _, host := range virtualservice.Spec.Hosts {
		if host == "" || host == "*" {
			continue
		}

		parts := strings.Split(host, "/")

		// If the input hostname is of the form my-namespace/foo.bar.com, remove the namespace
		// before appending it to the list of endpoints to create
		if len(parts) == 2 {
			host = parts[1]
		}

		hosts = append(hosts, host)
	}

	hostnameList := getHostnamesFromAnnotations(virtualservice.Annotations)
	for _, host := range hostnameList {
		hosts = append(hosts, host)
	}

	for _, host := range hosts {
		targetGateway, err := sc.targetGatewayForService(ctx, virtualservice, host)

		if err != nil {
			return endpoints, err
		}

		// No target. Internal service. Return empty list
		if targetGateway == "" {
			continue
		}

		endpoints = append(endpoints, registry.NewEndpoint(host, resource))
	}

	return endpoints, nil
}

func parseGateway(gateway string) (namespace, name string, err error) {
	parts := strings.Split(gateway, "/")
	if len(parts) == 2 {
		namespace, name = parts[0], parts[1]
	} else if len(parts) == 1 {
		name = parts[0]
	} else {
		err = fmt.Errorf("invalid gateway name (name or namespace/name) found '%v'", gateway)
	}

	return
}

func (sc *virtualServiceSource) targetGatewayForService(ctx context.Context, virtualService *networkingv1alpha3.VirtualService, vsHost string) (string, error) {
	// for each host we need to iterate through the gateways because each host might match for only one of the gateways
	for _, gateway := range virtualService.Spec.Gateways {
		gateway, err := sc.getGateway(ctx, gateway, virtualService)
		if err != nil {
			return "", err
		}
		if gateway == nil {
			continue
		}
		if !virtualServiceBindsToGateway(virtualService, gateway, vsHost) {
			continue
		}

		return gateway.Name, nil
	}

	return "", nil
}

func virtualServiceBindsToGateway(virtualService *networkingv1alpha3.VirtualService, gateway *networkingv1alpha3.Gateway, vsHost string) bool {
	isValid := false
	if len(virtualService.Spec.ExportTo) == 0 {
		isValid = true
	} else {
		for _, ns := range virtualService.Spec.ExportTo {
			if ns == "*" || ns == gateway.Namespace || (ns == "." && gateway.Namespace == virtualService.Namespace) {
				isValid = true
			}
		}
	}
	if !isValid {
		return false
	}

	for _, server := range gateway.Spec.Servers {
		for _, host := range server.Hosts {
			namespace := "*"
			parts := strings.Split(host, "/")
			if len(parts) == 2 {
				namespace = parts[0]
				host = parts[1]
			} else if len(parts) != 1 {
				log.Debugf("Gateway %s/%s has invalid host %s", gateway.Namespace, gateway.Name, host)
				continue
			}

			if namespace == "*" || namespace == virtualService.Namespace || (namespace == "." && virtualService.Namespace == gateway.Namespace) {
				if host == "*" {
					return true
				}

				suffixMatch := false
				if strings.HasPrefix(host, "*.") {
					suffixMatch = true
				}

				if host == vsHost || (suffixMatch && strings.HasSuffix(vsHost, host[1:])) {
					return true
				}
			}
		}
	}

	return false
}
