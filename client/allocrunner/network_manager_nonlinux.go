//+build !linux

package allocrunner

import (
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// TODO: Support windows shared networking
func newNetworkManager(alloc *structs.Allocation, driverManager drivermanager.Manager) (nm drivers.DriverNetworkManager, err error) {
	return nil, nil
}
