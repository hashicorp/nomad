package structs

import "time"

// Volume is a representation of a persistent storage volume from an external
// provider
type Volume struct {
	ID       string
	Provider string
}

// StorageInfo is the current state of a single storage provider. This is
// updated regularly as provider health changes on the node.
type StorageInfo struct {
	Attributes        map[string]string
	Detected          bool
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time
}
