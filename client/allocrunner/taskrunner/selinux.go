// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
)

// DriverNeedsLabeledSockets reports whether tasks of a driver with these
// capabilities need the unix sockets they connect to created with the
// container SELinux type, when the client.selinux.label_task_sockets option is
// set.
//
// Only drivers with image based filesystem isolation (such as Docker, podman
// and containerd) run tasks as confined containers. Other drivers (such as
// exec, raw_exec, java and qemu) run tasks in the agent's own domain, which can
// already connect to the agent's sockets, so their sockets are left alone.
func DriverNeedsLabeledSockets(caps *drivers.Capabilities) bool {
	return caps != nil && caps.FSIsolation == fsisolation.Image
}
