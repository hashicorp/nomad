//+build !linux

package allocrunner

import (
	hclog "github.com/hashicorp/go-hclog"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// TODO: Support windows shared networking
func newNetworkManager(alloc *structs.Allocation, driverManager drivermanager.Manager) (nm drivers.DriverNetworkManager, err error) {
	return nil, nil
}

func newNetworkConfigurator(log hclog.Logger, alloc *structs.Allocation, config *clientconfig.Config) (NetworkConfigurator, error) {
	return &hostNetworkConfigurator{}, nil
}
