package registry

import "fmt"

type Endpoint struct {
	Host     string
	Resource string
}

func NewEndpoint(host string, resource string) *Endpoint {
	return &Endpoint{Resource: resource, Host: host}
}

func (e Endpoint) String() string {
	return fmt.Sprintf("Endpoint[Host:%s][Resoure:%s]", e.Host, e.Resource)
}
