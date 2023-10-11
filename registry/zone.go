package registry

const RegistryRecordType = "TXT"

type Zone struct {
	Name  string
	Hosts []*Host
}

func NewZone(name string) *Zone {
	return &Zone{Name: name, Hosts: make([]*Host, 0)}
}

func (z *Zone) IsHostRecordType(recordType string) bool {
	switch recordType {
	case "A", "AAAA", "CNAME":
		return true
	default:
		return false
	}
}

func (z *Zone) IsRegistryRecordType(recordType string) bool {
	return recordType == RegistryRecordType
}

func (z *Zone) AddHost(record *Host) {
	z.Hosts = append(z.Hosts, record)
}
