package structs

import (
	"time"

	"github.com/hashicorp/nomad/helper"
)

// StoragePluginInfo is the current state of a single storage plugin. This
// is updated as regularly as plugin health changes on the node
// TODO: Add support for NodeTopology.
type StoragePluginInfo struct {
	Attributes        map[string]string
	Detected          bool
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time

	// NodeID is the ID of the Node in the external storage provider as returned
	// by the CSI NodePlugin.NodeGetInfo() RPC.
	NodeID string

	// MaxVolumeCount is the number of volumes that the plugin can mount to the
	// Node as returned by the CSI NodePlugin.NodeGetInfo() RPC.
	MaxVolumeCount int64
}

func (s *StoragePluginInfo) Copy() *StoragePluginInfo {
	if s == nil {
		return nil
	}

	c := new(StoragePluginInfo)
	*c = *s
	c.Attributes = helper.CopyMapStringString(s.Attributes)
	return c
}

// DriverInfo is the current state of a single driver. This is updated
// regularly as driver health changes on the node.
type DriverInfo struct {
	Attributes        map[string]string
	Detected          bool
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time
}

func (di *DriverInfo) Copy() *DriverInfo {
	if di == nil {
		return nil
	}

	cdi := new(DriverInfo)
	*cdi = *di
	cdi.Attributes = helper.CopyMapStringString(di.Attributes)
	return cdi
}

// MergeHealthCheck merges information from a health check for a drier into a
// node's driver info
func (di *DriverInfo) MergeHealthCheck(other *DriverInfo) {
	di.Healthy = other.Healthy
	di.HealthDescription = other.HealthDescription
	di.UpdateTime = other.UpdateTime
}

// MergeFingerprint merges information from fingerprinting a node for a driver
// into a node's driver info for that driver.
func (di *DriverInfo) MergeFingerprintInfo(other *DriverInfo) {
	di.Detected = other.Detected
	di.Attributes = other.Attributes
}

// DriverInfo determines if two driver info objects are equal..As this is used
// in the process of health checking, we only check the fields that are
// computed by the health checker. In the future, this will be merged.
func (di *DriverInfo) HealthCheckEquals(other *DriverInfo) bool {
	if di == nil && other == nil {
		return true
	}

	if di.Healthy != other.Healthy {
		return false
	}

	if di.HealthDescription != other.HealthDescription {
		return false
	}

	return true
}
