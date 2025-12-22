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

// matchesAnyLabel returns true if the given labels map has at least one label
// matching any of the provided selectors. Selectors are strings in the form
// "key:value" or "key=value". Matching uses OR semantics across selectors.
// If selectors is empty, returns true (no filter applied).
func matchesAnyLabel(labelsMap map[string]string, selectors []string) bool {
	if len(selectors) == 0 {
		return true
	}
	if labelsMap == nil {
		return false
	}

	for _, sel := range selectors {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}

		var key, val string
		if parts := strings.SplitN(sel, ":", 2); len(parts) == 2 {
			key = strings.TrimSpace(parts[0])
			val = strings.TrimSpace(parts[1])
		} else if parts := strings.SplitN(sel, "=", 2); len(parts) == 2 {
			key = strings.TrimSpace(parts[0])
			val = strings.TrimSpace(parts[1])
		} else {
			// If selector has no separator, treat it as key-only and just check presence.
			key = sel
			val = ""
		}

		if key == "" {
			continue
		}

		if labelVal, ok := labelsMap[key]; ok {
			if val == "" || labelVal == val {
				return true
			}
		}
	}

	return false
}
