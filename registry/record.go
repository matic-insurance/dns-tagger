package registry

import "strings"

const ExternalDnsIdentifier = "heritage=external-dns"
const OwnerId = "external-dns/owner="
const ResourceId = "external-dns/resource="

type Record struct {
	Zone     Zone
	info     string
	Name     string
	Owner    string
	Resource string
}

func (r Record) Info() string {
	return r.info
}

func NewRecord(zone Zone, name string, info string) Record {
	owner, resource := parseInfo(info)
	return Record{Zone: zone, Name: name, Owner: owner, Resource: resource, info: info}
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
