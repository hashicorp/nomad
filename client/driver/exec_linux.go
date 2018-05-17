package driver

import (
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"golang.org/x/sys/unix"
)

const (
	// The key populated in Node Attributes to indicate the presence of the Exec
	// driver
	execDriverAttr = "driver.exec"
)

func (d *ExecDriver) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	// The exec driver will be detected in every case
	resp.Detected = true

	// Only enable if cgroups are available and we are root
	if !cgroupsMounted(req.Node) {
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Printf("[INFO] driver.exec: cgroups unavailable, disabling")
		}
		d.fingerprintSuccess = helper.BoolToPtr(false)
		resp.RemoveAttribute(execDriverAttr)
		return nil
	} else if unix.Geteuid() != 0 {
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Printf("[DEBUG] driver.exec: must run as root user, disabling")
		}
		d.fingerprintSuccess = helper.BoolToPtr(false)
		resp.RemoveAttribute(execDriverAttr)
		return nil
	}

	if d.fingerprintSuccess == nil || !*d.fingerprintSuccess {
		d.logger.Printf("[DEBUG] driver.exec: exec driver is enabled")
	}
	resp.AddAttribute(execDriverAttr, "1")
	d.fingerprintSuccess = helper.BoolToPtr(true)
	return nil
}
