//+build !windows

package driver

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/moby/moby/daemon/caps"
)

const (
	// Setting default network mode for non-windows OS as bridge
	defaultNetworkMode = "bridge"
)

func getPortBinding(ip string, port string) []docker.PortBinding {
	return []docker.PortBinding{{HostIP: ip, HostPort: port}}
}

func tweakCapabilities(basics, adds, drops []string) ([]string, error) {
	// Moby mixes 2 different capabilities formats: prefixed with "CAP_"
	// and not. We do the conversion here to have a consistent,
	// non-prefixed format on the Nomad side.
	for i, cap := range basics {
		basics[i] = "CAP_" + cap
	}

	effectiveCaps, err := caps.TweakCapabilities(basics, adds, drops)
	if err != nil {
		return effectiveCaps, err
	}

	for i, cap := range effectiveCaps {
		effectiveCaps[i] = cap[len("CAP_"):]
	}
	return effectiveCaps, nil
}
