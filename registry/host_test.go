package registry

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHost_IsManaged_WhenHaveRegistryRecords(t *testing.T) {
	host := NewHost("test.example.com", "CNAME", "test")
	host.AddRegistryRecord(NewRecord("prefix-test.example.com", ""))
	assert.Equal(t, true, host.IsManaged())
}

func TestHost_IsManaged_NoRegistryRecords(t *testing.T) {
	host := NewHost("test.example.com", "CNAME", "test")
	assert.Equal(t, false, host.IsManaged())
}
