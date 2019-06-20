package allocrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/libcni"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	// envCNIPath is the environment variable name to use to derive the CNI path
	// when it is not explicitly set by the client
	envCNIPath = "CNI_PATH"

	// defaultCNIPath is the CNI path to use when it is not set by the client
	// and is not set by environment variable
	defaultCNIPath = "/opt/cni/bin"

	// defaultNomadBridgeName is the name of the bridge to use when not set by
	// the client
	defaultNomadBridgeName = "nomad"

	// bridgeNetworkAllocIfName is the name that is set for the interface created
	// inside of the alloc network which is connected to the bridge
	bridgeNetworkContainerIfName = "eth0"

	// defaultNomadAllocSubnet is the subnet to use for host local ip address
	// allocation when not specified by the client
	defaultNomadAllocSubnet = "172.26.66.0/23"
)

// bridgeNetworkConfigurator is a NetworkConfigurator which adds the alloc to a
// shared bridge, configures masquerading for egress traffic and port mapping
// for ingress
type bridgeNetworkConfigurator struct {
	ctx         context.Context
	cniConfig   *libcni.CNIConfig
	allocSubnet string
	bridgeName  string
}

func newBridgeNetworkConfigurator(ctx context.Context, bridgeName, ipRange, cniPath string) *bridgeNetworkConfigurator {
	b := &bridgeNetworkConfigurator{
		ctx:         ctx,
		bridgeName:  bridgeName,
		allocSubnet: ipRange,
	}
	if cniPath == "" {
		if cniPath = os.Getenv(envCNIPath); cniPath == "" {
			cniPath = defaultCNIPath
		}
	}
	b.cniConfig = libcni.NewCNIConfig(filepath.SplitList(cniPath), nil)

	if b.bridgeName == "" {
		b.bridgeName = defaultNomadBridgeName
	}

	if b.allocSubnet == "" {
		b.allocSubnet = defaultNomadAllocSubnet
	}

	return b
}

// Setup calls the CNI plugins with the add action
func (b *bridgeNetworkConfigurator) Setup(alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	netconf, err := b.buildNomadNetConfig()
	if err != nil {
		return err
	}

	spew.Dump(netconf)

	result, err := b.cniConfig.AddNetworkList(b.ctx, netconf, b.runtimeConf(alloc, spec))
	if result != nil {
		result.Print()
	}

	return err

}

// Teardown calls the CNI plugins with the delete action
func (b *bridgeNetworkConfigurator) Teardown(alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	netconf, err := b.buildNomadNetConfig()
	if err != nil {
		return err
	}

	err = b.cniConfig.DelNetworkList(b.ctx, netconf, b.runtimeConf(alloc, spec))
	return err

}

// getPortMapping builds a list of portMapping structs that are used as the
// portmapping capability arguments for the portmap CNI plugin
func getPortMapping(alloc *structs.Allocation) []*portMapping {
	ports := []*portMapping{}
	for _, network := range alloc.AllocatedResources.Shared.Networks {
		for _, port := range append(network.DynamicPorts, network.ReservedPorts...) {
			for _, proto := range []string{"tcp", "udp"} {
				ports = append(ports, &portMapping{
					Host:      port.Value,
					Container: port.To,
					Proto:     proto,
				})
			}
		}
	}
	return ports
}

// portMapping is the json representation of the portmapping capability arguments
// for the portmap CNI plugin
type portMapping struct {
	Host      int    `json:"hostPort"`
	Container int    `json:"containerPort"`
	Proto     string `json:"protocol"`
}

// runtimeConf builds the configuration needed by CNI to locate the target netns
func (b *bridgeNetworkConfigurator) runtimeConf(alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) *libcni.RuntimeConf {
	return &libcni.RuntimeConf{
		ContainerID: fmt.Sprintf("nomad-%s", alloc.ID[:8]),
		NetNS:       spec.Path,
		IfName:      bridgeNetworkContainerIfName,
		CapabilityArgs: map[string]interface{}{
			"portMappings": getPortMapping(alloc),
		},
	}
}

// buildNomadNetConfig generates the CNI network configuration for the bridge
// networking mode
func (b *bridgeNetworkConfigurator) buildNomadNetConfig() (*libcni.NetworkConfigList, error) {
	rendered := fmt.Sprintf(nomadCNIConfigTemplate, b.bridgeName, b.allocSubnet)
	return libcni.ConfListFromBytes([]byte(rendered))
}

const nomadCNIConfigTemplate = `{
	"cniVersion": "0.4.0",
	"name": "nomad",
	"plugins": [
		{
			"type": "bridge",
			"bridge": "%s",
			"isDefaultGateway": true,
			"ipMasq": true,
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{
							"subnet": "%s"
						}
					]
				]
			}
		},
		{
			"type": "firewall"
		},
		{
			"type": "portmap",
			"capabilities": {"portMappings": true}
		}
	]
}
`
