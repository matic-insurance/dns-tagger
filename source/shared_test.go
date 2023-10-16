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
	"github.com/matic-insurance/dns-tager/registry"
	"sort"
	"testing"
)

const namespace = "testing"

func sortEndpoints(endpoints []*registry.Endpoint) {
	sort.Slice(endpoints, func(i, k int) bool {
		// Sort by DNSName, RecordType, and Targets
		ei, ek := endpoints[i], endpoints[k]
		if ei.Host != ek.Host {
			return ei.Host < ek.Host
		}
		if ei.Resource != ek.Resource {
			return ei.Resource < ek.Resource
		}

		return false
	})
}

func validateEndpoints(t *testing.T, endpoints, expected []*registry.Endpoint) {
	t.Helper()

	if len(endpoints) != len(expected) {
		t.Fatalf("expected %d endpoints, got %d", len(expected), len(endpoints))
	}

	// Make sure endpoints are sorted - validateEndpoint() depends on it.
	sortEndpoints(endpoints)
	sortEndpoints(expected)

	for i := range endpoints {
		validateEndpoint(t, endpoints[i], expected[i])
	}
}

func validateEndpoint(t *testing.T, endpoint, expected *registry.Endpoint) {
	t.Helper()

	if endpoint.Host != expected.Host {
		t.Errorf("DNSName expected %q, got %q", expected.Host, endpoint.Host)
	}

	if endpoint.Resource != expected.Resource {
		t.Errorf("Resource expected %v, got %v", expected.Resource, endpoint.Resource)
	}
}
