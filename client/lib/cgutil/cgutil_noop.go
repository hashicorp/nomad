//go:build !linux

package cgutil

import (
	"github.com/hashicorp/go-hclog"
)

const (
	// DefaultCgroupParent does not apply to non-Linux operating systems.
	DefaultCgroupParent = ""
)

// UseV2 is always false on non-Linux systems.
//
// This is a read-only value.
var UseV2 = false

// CreateCPUSetManager creates a no-op CpusetManager for non-Linux operating systems.
func CreateCPUSetManager(string, []uint16, hclog.Logger) CpusetManager {
	return new(NoopCpusetManager)
}

// FindCgroupMountpointDir returns nothing for non-Linux operating systems.
func FindCgroupMountpointDir() (string, error) {
	return "", nil
}

// GetCgroupParent returns nothing for non-Linux operating systems.
func GetCgroupParent(string) string {
	return DefaultCgroupParent
}

// GetCPUsFromCgroup returns nothing for non-Linux operating systems.
func GetCPUsFromCgroup(string) ([]uint16, error) {
	return nil, nil
}

// CgroupScope returns nothing for non-Linux operating systems.
func CgroupScope(allocID, task string) string {
	return ""
}
