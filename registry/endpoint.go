package registry

type Endpoint struct {
	Resource string
	Host     string
	Targets  []string
}

func NewEndpoint(resource string, host string, targets []string) Endpoint {
	return Endpoint{Resource: resource, Host: host, Targets: targets}
}
