package source

import (
	"context"
	"fmt"
	"github.com/matic-insurance/dns-tager/registry"
	"reflect"
	"strings"
	"time"
)

const (
	controllerAnnotationKey   = "external-dns.alpha.kubernetes.io/controller"
	controllerAnnotationValue = "dns-controller"
	hostnameAnnotationKey     = "external-dns.alpha.kubernetes.io/hostname"
)

type Source interface {
	Endpoints(ctx context.Context) ([]*registry.Endpoint, error)
	// AddEventHandler adds an event handler that should be triggered if something in source changes
	AddEventHandler(context.Context, func())
}

func getHostnamesFromAnnotations(annotations map[string]string) []string {
	hostnameAnnotation, exists := annotations[hostnameAnnotationKey]
	if !exists {
		return nil
	}
	return splitHostnameAnnotation(hostnameAnnotation)
}

func splitHostnameAnnotation(annotation string) []string {
	return strings.Split(strings.Replace(annotation, " ", "", -1), ",")
}

type eventHandlerFunc func()

func (fn eventHandlerFunc) OnAdd(_ interface{}, _ bool) { fn() }
func (fn eventHandlerFunc) OnUpdate(_, _ interface{})   { fn() }
func (fn eventHandlerFunc) OnDelete(_ interface{})      { fn() }

type informerFactory interface {
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

func waitForCacheSync(ctx context.Context, factory informerFactory) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for typ, done := range factory.WaitForCacheSync(ctx.Done()) {
		if !done {
			select {
			case <-ctx.Done():
				return fmt.Errorf("failed to sync %v: %v", typ, ctx.Err())
			default:
				return fmt.Errorf("failed to sync %v", typ)
			}
		}
	}
	return nil
}
