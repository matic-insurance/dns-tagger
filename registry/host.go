package registry

import "fmt"

type Host struct {
	Name            string
	RecordType      string
	Value           string
	RegistryRecords []*Record
}

func (h *Host) String() string {
	if h.IsManaged() {
		return fmt.Sprintf("Host[Managed][Dns:%s]", h.Name)
	} else {
		return fmt.Sprintf("Host[Unmanaged][Dns:%s]", h.Name)
	}
}

func (h *Host) AddRegistryRecord(record *Record) {
	h.RegistryRecords = append(h.RegistryRecords, record)
}

func (h *Host) IsManaged() bool {
	return len(h.RegistryRecords) > 0
}

func NewHost(name string, recordType string, value string) *Host {
	return &Host{Name: name, RecordType: recordType, Value: value, RegistryRecords: make([]*Record, 0)}
}
