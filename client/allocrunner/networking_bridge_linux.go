package allocrunner

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	cni "github.com/containerd/go-cni"
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

	// bridgeNetworkAllocIfPrefix is the prefix that is used for the interface
	// name created inside of the alloc network which is connected to the bridge
	bridgeNetworkAllocIfPrefix = "eth"

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
	cni         cni.CNI
	allocSubnet string
	bridgeName  string

	rand   *rand.Rand
	logger hclog.Logger
}

func newBridgeNetworkConfigurator(log hclog.Logger, bridgeName, ipRange, cniPath string) (*bridgeNetworkConfigurator, error) {
	b := &bridgeNetworkConfigurator{
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

	c, err := cni.New(cni.WithPluginDir(filepath.SplitList(cniPath)),
		cni.WithInterfacePrefix(bridgeNetworkAllocIfPrefix))
	if err != nil {
		return nil, err
	}
	b.cni = c

	if b.bridgeName == "" {
		b.bridgeName = defaultNomadBridgeName
	}

	if b.allocSubnet == "" {
		b.allocSubnet = defaultNomadAllocSubnet
	}

	return b, nil
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
func (b *bridgeNetworkConfigurator) Setup(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	if err := b.ensureForwardingRules(); err != nil {
		return fmt.Errorf("failed to initialize table forwarding rules: %v", err)
	}

	if err := b.cni.Load(cni.WithConfListBytes(b.buildNomadNetConfig())); err != nil {
		return err
	}

	// Depending on the version of bridge cni plugin used, a known race could occure
	// where two alloc attempt to create the nomad bridge at the same time, resulting
	// in one of them to fail. This rety attempts to overcome any
	const retry = 3
	for attempt := 1; ; attempt++ {
		//TODO eventually returning the IP from the result would be nice to have in the alloc
		if _, err := b.cni.Setup(ctx, alloc.ID, spec.Path, cni.WithCapabilityPortMap(getPortMapping(alloc))); err != nil {
			b.logger.Warn("failed to configure bridge network", "err", err, "attempt", attempt)
			if attempt == retry {
				return fmt.Errorf("failed to configure bridge network: %v", err)
			}
			// Sleep for 1 second + jitter
			time.Sleep(time.Second + (time.Duration(b.rand.Int63n(1000)) * time.Millisecond))
			continue
		}
		break
	}

	return nil

}

// Teardown calls the CNI plugins with the delete action
func (b *bridgeNetworkConfigurator) Teardown(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	return b.cni.Remove(ctx, alloc.ID, spec.Path, cni.WithCapabilityPortMap(getPortMapping(alloc)))
}

// getPortMapping builds a list of portMapping structs that are used as the
// portmapping capability arguments for the portmap CNI plugin
func getPortMapping(alloc *structs.Allocation) []cni.PortMapping {
	ports := []cni.PortMapping{}
	for _, network := range alloc.AllocatedResources.Shared.Networks {
		for _, port := range append(network.DynamicPorts, network.ReservedPorts...) {
			if port.To < 1 {
				continue
			}
			for _, proto := range []string{"tcp", "udp"} {
				ports = append(ports, cni.PortMapping{
					HostPort:      int32(port.Value),
					ContainerPort: int32(port.To),
					Protocol:      proto,
				})
			}
		}
	}
	return ports
}

func (b *bridgeNetworkConfigurator) buildNomadNetConfig() []byte {
	return []byte(fmt.Sprintf(nomadCNIConfigTemplate, b.bridgeName, b.allocSubnet, cniAdminChainName))
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
			"forceAddress": true,
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
