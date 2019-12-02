package structs

import (
	"reflect"
	"time"

	"github.com/hashicorp/nomad/helper"
)

// CSINodeInfo is the fingerprinted data from a CSI Plugin that is specific to
// the Node API.
type CSINodeInfo struct {
	// ID is the identity of a given nomad client as observed by the storage
	// provider.
	ID string

	Topology map[string]string
}

func (n *CSINodeInfo) Copy() *CSINodeInfo {
	if n == nil {
		return nil
	}

	nc := new(CSINodeInfo)
	*nc = *n
	nc.Topology = helper.CopyMapStringString(n.Topology)

	return nc
}

// CSIControllerInfo is the fingerprinted data from a CSI Plugin that is specific to
// the Controller API.
type CSIControllerInfo struct {
	Capabilities []string
}

// CSIInfo is the current state of a single CSI Plugin. This is updated regularly
// as plugin health changes on the node.
type CSIInfo struct {
	PluginID          string
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time

	// RequiresControllerPlugin is set when the CSI Plugin returns the
	// CONTROLLER_SERVICE capability. When this is true, the volumes should not be
	// scheduled on this client until a matching controller plugin is available.
	RequiresControllerPlugin bool

	// RequiresTopologies is set when the CSI Plugin returns the
	// VOLUME_ACCESSIBLE_CONSTRAINTS capability. When this is true, we must
	// respect the Volume and Node Topology information.
	RequiresTopologies bool

	// CSI Specific metadata
	ControllerInfo *CSIControllerInfo `json:",omitempty"`
	NodeInfo       *CSINodeInfo       `json:",omitempty"`
}

func (c *CSIInfo) Copy() *CSIInfo {
	if c == nil {
		return nil
	}

	nc := new(CSIInfo)
	*nc = *c
	nc.NodeInfo = c.NodeInfo.Copy()

	return nc
}

func (c *CSIInfo) IsEqual(o *CSIInfo) bool {
	nc := *c
	nc.UpdateTime = time.Time{}
	no := *o
	no.UpdateTime = time.Time{}
	return reflect.DeepEqual(nc, no)
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
