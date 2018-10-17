//+build !linux

package exec

import "github.com/hashicorp/nomad/plugins/drivers"

func (d *ExecDriver) buildFingerprint() *drivers.Fingerprint {
	return &drivers.Fingerprint{
		Health:            drivers.HealthStateUndetected,
		HealthDescription: "exec driver unsupported on client OS",
	}
}
