package registry

import (
	"fmt"
	"strings"
)

const ExternalDnsIdentifier = "heritage=external-dns"
const OwnerId = "external-dns/owner="
const ResourceId = "external-dns/resource="

type Record struct {
	Name     string
	Owner    string
	Resource string
}

func (r Record) Info() string {
	segments := []string{ExternalDnsIdentifier, OwnerId + r.Owner, ResourceId + r.Resource}
	return strings.Join(segments, ",")
}

func (r Record) IsManaging(host *Host) bool {
	if r.Name == host.Name || strings.HasPrefix(r.Name, host.Name) || strings.HasSuffix(r.Name, host.Name) {
		return true
	} else {
		return false
	}
}

func (r Record) NewOwner(ownerId string) *Record {
	return &Record{Name: r.Name, Owner: ownerId, Resource: r.Resource}
}

func (r Record) NewResource(resource string) *Record {
	return &Record{Name: r.Name, Owner: r.Owner, Resource: resource}
}

func (r Record) String() string {
	return fmt.Sprintf("RegistryRecord[Host:%s][Owner:%s][Resource:%s]", r.Name, r.Owner, r.Resource)
}

func NewRecord(name string, info string) *Record {
	owner, resource := parseInfo(info)
	return &Record{Name: name, Owner: owner, Resource: resource}
}

func parseInfo(info string) (string, string) {
	owner, resource := "", ""
	for _, segment := range strings.Split(info, ",") {
		if strings.HasPrefix(segment, OwnerId) {
			owner = strings.TrimPrefix(segment, OwnerId)
		} else if strings.HasPrefix(segment, ResourceId) {
			resource = strings.TrimPrefix(segment, ResourceId)
		}
	}
	return owner, resource
}
