package structs

import (
	"strings"
	"time"
)

// DriverInfo is the current state of a single driver. This is updated
// regularly as driver health changes on the node.
type DriverInfo struct {
	Attributes        map[string]string
	Detected          bool
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time
}

func (di *DriverInfo) MergeHealthCheck(other *DriverInfo) {
	di.Healthy = other.Healthy
	di.HealthDescription = other.HealthDescription
	di.UpdateTime = other.UpdateTime
}

func (di *DriverInfo) MergeFingerprintInfo(other *DriverInfo) {
	di.Detected = other.Detected
	di.Attributes = other.Attributes
}

// DriverInfo determines if two driver info objects are equal..As this is used
// in the process of health checking, we only check the fields that are
// computed by the health checker. In the future, this will be merged.
func (di *DriverInfo) HealthCheckEquals(other *DriverInfo) bool {
	if di == nil && other != nil || di != nil && other == nil {
		return false
	}

	if di.Healthy != other.Healthy {
		return false
	}

	if strings.Compare(di.HealthDescription, other.HealthDescription) != 0 {
		return false
	}

	return true
}
