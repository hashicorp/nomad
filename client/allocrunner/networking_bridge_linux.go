package allocrunner

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/coreos/go-iptables/iptables"
	hclog "github.com/hashicorp/go-hclog"
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
	defaultNomadAllocSubnet = "172.26.64.0/20" // end 172.26.79.255

	// cniAdminChainName is the name of the admin iptables chain used to allow
	// forwarding traffic to allocations
	cniAdminChainName = "NOMAD-ADMIN"
)

// bridgeNetworkConfigurator is a NetworkConfigurator which adds the alloc to a
// shared bridge, configures masquerading for egress traffic and port mapping
// for ingress
type bridgeNetworkConfigurator struct {
	ctx         context.Context
	cniConfig   *libcni.CNIConfig
	allocSubnet string
	bridgeName  string

	rand   *rand.Rand
	logger hclog.Logger
}

func newBridgeNetworkConfigurator(log hclog.Logger, ctx context.Context, bridgeName, ipRange, cniPath string) *bridgeNetworkConfigurator {
	b := &bridgeNetworkConfigurator{
		ctx:         ctx,
		bridgeName:  bridgeName,
		allocSubnet: ipRange,
		rand:        rand.New(rand.NewSource(time.Now().Unix())),
		logger:      log,
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

// ensureForwardingRules ensures that a forwarding rule is added to iptables
// to allow traffic inbound to the bridge network
// // ensureForwardingRules ensures that a forwarding rule is added to iptables
// to allow traffic inbound to the bridge network
func (b *bridgeNetworkConfigurator) ensureForwardingRules() error {
	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	if err = ensureChain(ipt, "filter", cniAdminChainName); err != nil {
		return err
	}

	if err := ensureFirstChainRule(ipt, cniAdminChainName, b.generateAdminChainRule()); err != nil {
		return err
	}

	return nil
}

// ensureChain ensures that the given chain exists, creating it if missing
func ensureChain(ipt *iptables.IPTables, table, chain string) error {
	chains, err := ipt.ListChains(table)
	if err != nil {
		return fmt.Errorf("failed to list iptables chains: %v", err)
	}
	for _, ch := range chains {
		if ch == chain {
			return nil
		}
	}

	err = ipt.NewChain(table, chain)

	// if err is for chain already existing return as it is possible another
	// goroutine created it first
	if e, ok := err.(*iptables.Error); ok && e.ExitStatus() == 1 {
		return nil
	}

	return err
}

// ensureFirstChainRule ensures the given rule exists as the first rule in the chain
func ensureFirstChainRule(ipt *iptables.IPTables, chain string, rule []string) error {
	exists, err := ipt.Exists("filter", chain, rule...)
	if !exists && err == nil {
		// iptables rules are 1-indexed
		err = ipt.Insert("filter", chain, 1, rule...)
	}
	return err
}

// generateAdminChainRule builds the iptables rule that is inserted into the
// CNI admin chain to ensure traffic forwarding to the bridge network
func (b *bridgeNetworkConfigurator) generateAdminChainRule() []string {
	return []string{"-o", b.bridgeName, "-d", b.allocSubnet, "-j", "ACCEPT"}
}

// Setup calls the CNI plugins with the add action
func (b *bridgeNetworkConfigurator) Setup(alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	if err := b.ensureForwardingRules(); err != nil {
		return fmt.Errorf("failed to initialize table forwarding rules: %v", err)
	}

	netconf, err := b.buildNomadNetConfig()
	if err != nil {
		return err
	}

	// Depending on the version of bridge cni plugin used, a known race could occure
	// where two alloc attempt to create the nomad bridge at the same time, resulting
	// in one of them to fail. This rety attempts to overcome any
	const retry = 3
	for attempt := 1; ; attempt++ {
		_, err := b.cniConfig.AddNetworkList(b.ctx, netconf, b.runtimeConf(alloc, spec))
		if err == nil {
			break
		}

		b.logger.Warn("failed to configure bridge network", "err", err, "attempt", attempt)
		if attempt == retry {
			return err
		}

		// Sleep for 1 second + jitter
		time.Sleep(time.Second + (time.Duration(b.rand.Int63n(1000)) * time.Millisecond))
	}

	return nil

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
			if port.To < 1 {
				continue
			}
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
	rendered := fmt.Sprintf(nomadCNIConfigTemplate, b.bridgeName, b.allocSubnet, cniAdminChainName)
	return libcni.ConfListFromBytes([]byte(rendered))
}

const nomadCNIConfigTemplate = `{
	"cniVersion": "0.4.0",
	"name": "nomad",
	"plugins": [
		{
			"type": "bridge",
			"bridge": "%s",
			"ipMasq": true,
			"isGateway": true,
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{
							"subnet": "%s"
						}
					]
				],
				"routes": [
					{ "dst": "0.0.0.0/0" }
				]
			}
		},
		{
			"type": "firewall",
			"backend": "iptables",
			"iptablesAdminChainName": "%s"
		},
		{
			"type": "portmap",
			"capabilities": {"portMappings": true},
			"snat": true
		}
	]
}
`
