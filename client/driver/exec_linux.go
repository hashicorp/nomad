package driver

import (
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/sys/unix"
)

func (d *ExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if cgroups are available and we are root
	if !cgroupsMounted(node) {
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Printf("[DEBUG] driver.exec: cgroups unavailable, disabling")
		}
		d.fingerprintSuccess = helper.BoolToPtr(false)
		delete(node.Attributes, execDriverAttr)
		return false, nil
	} else if unix.Geteuid() != 0 {
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Printf("[DEBUG] driver.exec: must run as root user, disabling")
		}
		delete(node.Attributes, execDriverAttr)
		d.fingerprintSuccess = helper.BoolToPtr(false)
		return false, nil
	}

	if d.fingerprintSuccess == nil || !*d.fingerprintSuccess {
		d.logger.Printf("[DEBUG] driver.exec: exec driver is enabled")
	}
	node.Attributes[execDriverAttr] = "1"
	d.fingerprintSuccess = helper.BoolToPtr(true)
	return true, nil
}
