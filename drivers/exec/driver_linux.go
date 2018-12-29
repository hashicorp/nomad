package exec

import (
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/sys/unix"
)

func (d *ExecDriver) buildFingerprint() *drivers.Fingerprint {
	fp := &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: "healthy",
	}

	mount, err := fingerprint.FindCgroupMountpointDir()
	if err != nil {
		fp.Health = drivers.HealthStateUnhealthy
		fp.HealthDescription = "failed to discover cgroup mount point"
		d.logger.Warn(fp.HealthDescription, "error", err)
		return fp
	}

	if mount == "" {
		fp.Health = drivers.HealthStateUnhealthy
		fp.HealthDescription = "cgroups are unavailable"
		return fp
	}

	if unix.Geteuid() != 0 {
		fp.Health = drivers.HealthStateUnhealthy
		fp.HealthDescription = "exec driver must run as root"
		return fp
	}

	fp.Attributes["driver.exec"] = "1"
	return fp
}
