// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// For now CNI is supported only on Linux.
//
//go:build linux
// +build linux

package allocrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	cni "github.com/containerd/go-cni"
	cnilibrary "github.com/containernetworking/cni/libcni"
	"github.com/coreos/go-iptables/iptables"
	log "github.com/hashicorp/go-hclog"
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

	// defaultCNIInterfacePrefix is the network interface to use if not set in
	// client config
	defaultCNIInterfacePrefix = "eth"
)

type cniNetworkConfigurator struct {
	cni                     cni.CNI
	cniConf                 []byte
	ignorePortMappingHostIP bool

	rand   *rand.Rand
	logger log.Logger
}

func newCNINetworkConfigurator(logger log.Logger, cniPath, cniInterfacePrefix, cniConfDir, networkName string, ignorePortMappingHostIP bool) (*cniNetworkConfigurator, error) {
	cniConf, err := loadCNIConf(cniConfDir, networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to load CNI config: %v", err)
	}

	return newCNINetworkConfiguratorWithConf(logger, cniPath, cniInterfacePrefix, ignorePortMappingHostIP, cniConf)
}

func newCNINetworkConfiguratorWithConf(logger log.Logger, cniPath, cniInterfacePrefix string, ignorePortMappingHostIP bool, cniConf []byte) (*cniNetworkConfigurator, error) {
	conf := &cniNetworkConfigurator{
		cniConf:                 cniConf,
		rand:                    rand.New(rand.NewSource(time.Now().Unix())),
		logger:                  logger,
		ignorePortMappingHostIP: ignorePortMappingHostIP,
	}
	if cniPath == "" {
		if cniPath = os.Getenv(envCNIPath); cniPath == "" {
			cniPath = defaultCNIPath
		}
	}

	if cniInterfacePrefix == "" {
		cniInterfacePrefix = defaultCNIInterfacePrefix
	}

	c, err := cni.New(cni.WithPluginDir(filepath.SplitList(cniPath)),
		cni.WithInterfacePrefix(cniInterfacePrefix))
	if err != nil {
		return nil, err
	}
	conf.cni = c

	return conf, nil
}

// Setup calls the CNI plugins with the add action
func (c *cniNetworkConfigurator) Setup(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) (*structs.AllocNetworkStatus, error) {
	if err := c.ensureCNIInitialized(); err != nil {
		return nil, err
	}

	// Depending on the version of bridge cni plugin used, a known race could occure
	// where two alloc attempt to create the nomad bridge at the same time, resulting
	// in one of them to fail. This rety attempts to overcome those erroneous failures.
	const retry = 3
	var firstError error
	var res *cni.CNIResult
	for attempt := 1; ; attempt++ {
		var err error
		if res, err = c.cni.Setup(ctx, alloc.ID, spec.Path, cni.WithCapabilityPortMap(getPortMapping(alloc, c.ignorePortMappingHostIP))); err != nil {
			c.logger.Warn("failed to configure network", "error", err, "attempt", attempt)
			switch attempt {
			case 1:
				firstError = err
			case retry:
				return nil, fmt.Errorf("failed to configure network: %v", firstError)
			}

			// Sleep for 1 second + jitter
			time.Sleep(time.Second + (time.Duration(c.rand.Int63n(1000)) * time.Millisecond))
			continue
		}
		break
	}

	if c.logger.IsDebug() {
		resultJSON, _ := json.Marshal(res)
		c.logger.Debug("received result from CNI", "result", string(resultJSON))
	}

	return c.cniToAllocNet(res)

}

// cniToAllocNet converts a CNIResult to an AllocNetworkStatus or returns an
// error. The first interface and IP with a sandbox and address set are
// preferred. Failing that the first interface with an IP is selected.
func (c *cniNetworkConfigurator) cniToAllocNet(res *cni.CNIResult) (*structs.AllocNetworkStatus, error) {
	if len(res.Interfaces) == 0 {
		return nil, fmt.Errorf("failed to configure network: no interfaces found")
	}

	netStatus := new(structs.AllocNetworkStatus)

	// Unfortunately the go-cni library returns interfaces in an unordered map meaning
	// the results may be nondeterministic depending on CNI plugin output so make
	// sure we sort them by interface name.
	names := make([]string, 0, len(res.Interfaces))
	for k := range res.Interfaces {
		names = append(names, k)
	}
	sort.Strings(names)

	// Use the first sandbox interface with an IP address
	for _, name := range names {
		iface := res.Interfaces[name]
		if iface == nil {
			// this should never happen but this value is coming from external
			// plugins so we should guard against it
			delete(res.Interfaces, name)
			continue
		}

		if iface.Sandbox != "" && len(iface.IPConfigs) > 0 {
			netStatus.Address = iface.IPConfigs[0].IP.String()
			netStatus.InterfaceName = name
			break
		}
	}

	// If no IP address was found, use the first interface with an address
	// found as a fallback
	if netStatus.Address == "" {
		for _, name := range names {
			iface := res.Interfaces[name]
			if len(iface.IPConfigs) > 0 {
				ip := iface.IPConfigs[0].IP.String()
				c.logger.Debug("no sandbox interface with an address found CNI result, using first available", "interface", name, "ip", ip)
				netStatus.Address = ip
				netStatus.InterfaceName = name
				break
			}
		}
	}

	// If no IP address could be found, return an error
	if netStatus.Address == "" {
		return nil, fmt.Errorf("failed to configure network: no interface with an address")

	}

	// Use the first DNS results.
	if len(res.DNS) > 0 {
		netStatus.DNS = &structs.DNSConfig{
			Servers:  res.DNS[0].Nameservers,
			Searches: res.DNS[0].Search,
			Options:  res.DNS[0].Options,
		}
	}

	return netStatus, nil
}

func loadCNIConf(confDir, name string) ([]byte, error) {
	files, err := cnilibrary.ConfFiles(confDir, []string{".conf", ".conflist", ".json"})
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to detect CNI config file: %v", err)
	case len(files) == 0:
		return nil, fmt.Errorf("no CNI network config found in %s", confDir)
	}

	// files contains the network config files associated with cni network.
	// Use lexicographical way as a defined order for network config files.
	sort.Strings(files)
	for _, confFile := range files {
		if strings.HasSuffix(confFile, ".conflist") {
			confList, err := cnilibrary.ConfListFromFile(confFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load CNI config list file %s: %v", confFile, err)
			}
			if confList.Name == name {
				return confList.Bytes, nil
			}
		} else {
			conf, err := cnilibrary.ConfFromFile(confFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load CNI config file %s: %v", confFile, err)
			}
			if conf.Network.Name == name {
				return conf.Bytes, nil
			}
		}
	}

	return nil, fmt.Errorf("CNI network config not found for name %q", name)
}

// Teardown calls the CNI plugins with the delete action
func (c *cniNetworkConfigurator) Teardown(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	if err := c.ensureCNIInitialized(); err != nil {
		return err
	}

	if err := c.cni.Remove(ctx, alloc.ID, spec.Path, cni.WithCapabilityPortMap(getPortMapping(alloc, c.ignorePortMappingHostIP))); err != nil {
		// create a real handle to iptables
		ipt, iptErr := iptables.New()
		if iptErr != nil {
			return fmt.Errorf("failed to detect iptables: %w", iptErr)
		}
		// most likely the pause container was removed from underneath nomad
		return c.forceCleanup(ipt, alloc.ID)
	}

	return nil
}

// IPTables is a subset of iptables.IPTables
type IPTables interface {
	List(table, chain string) ([]string, error)
	Delete(table, chain string, rule ...string) error
	ClearAndDeleteChain(table, chain string) error
}

var (
	// ipRuleRe is used to parse a postrouting iptables rule created by nomad, e.g.
	//   -A POSTROUTING -s 172.26.64.191/32 -m comment --comment "name: \"nomad\" id: \"6b235529-8111-4bbe-520b-d639b1d2a94e\"" -j CNI-50e58ea77dc52e0c731e3799
	ipRuleRe = regexp.MustCompile(`-A POSTROUTING -s (\S+) -m comment --comment "name: \\"nomad\\" id: \\"([[:xdigit:]-]+)\\"" -j (CNI-[[:xdigit:]]+)`)
)

// forceCleanup is the backup plan for removing the iptables rule and chain associated with
// an allocation that was using bridge networking. The cni library refuses to handle a
// dirty state - e.g. the pause container is removed out of band, and so we must cleanup
// iptables ourselves to avoid leaking rules.
func (c *cniNetworkConfigurator) forceCleanup(ipt IPTables, allocID string) error {
	const (
		natTable         = "nat"
		postRoutingChain = "POSTROUTING"
		commentFmt       = `--comment "name: \"nomad\" id: \"%s\""`
	)

	// list the rules on the POSTROUTING chain of the nat table
	rules, err := ipt.List(natTable, postRoutingChain)
	if err != nil {
		return fmt.Errorf("failed to list iptables rules: %w", err)
	}

	// find the POSTROUTING rule associated with our allocation
	matcher := fmt.Sprintf(commentFmt, allocID)
	var ruleToPurge string
	for _, rule := range rules {
		if strings.Contains(rule, matcher) {
			ruleToPurge = rule
			break
		}
	}

	// no rule found for our allocation, just give up
	if ruleToPurge == "" {
		return fmt.Errorf("failed to find postrouting rule for alloc %s", allocID)
	}

	// re-create the rule we need to delete, as tokens
	subs := ipRuleRe.FindStringSubmatch(ruleToPurge)
	if len(subs) != 4 {
		return fmt.Errorf("failed to parse postrouting rule for alloc %s", allocID)
	}
	cidr := subs[1]
	id := subs[2]
	chainID := subs[3]
	toDel := []string{
		`-s`,
		cidr,
		`-m`,
		`comment`,
		`--comment`,
		`name: "nomad" id: "` + id + `"`,
		`-j`,
		chainID,
	}

	// remove the jump rule
	ok := true
	if err = ipt.Delete(natTable, postRoutingChain, toDel...); err != nil {
		c.logger.Warn("failed to remove iptables nat.POSTROUTING rule", "alloc_id", allocID, "chain", chainID, "error", err)
		ok = false
	}

	// remote the associated chain
	if err = ipt.ClearAndDeleteChain(natTable, chainID); err != nil {
		c.logger.Warn("failed to remove iptables nat chain", "chain", chainID, "error", err)
		ok = false
	}

	if !ok {
		return fmt.Errorf("failed to cleanup iptables rules for alloc %s", allocID)
	}

	return nil
}

func (c *cniNetworkConfigurator) ensureCNIInitialized() error {
	if err := c.cni.Status(); cni.IsCNINotInitialized(err) {
		return c.cni.Load(cni.WithConfListBytes(c.cniConf))
	} else {
		return err
	}
}

// getPortMapping builds a list of portMapping structs that are used as the
// portmapping capability arguments for the portmap CNI plugin
func getPortMapping(alloc *structs.Allocation, ignoreHostIP bool) []cni.PortMapping {
	var ports []cni.PortMapping

	if len(alloc.AllocatedResources.Shared.Ports) == 0 && len(alloc.AllocatedResources.Shared.Networks) > 0 {
		for _, network := range alloc.AllocatedResources.Shared.Networks {
			for _, port := range append(network.DynamicPorts, network.ReservedPorts...) {
				if port.To < 1 {
					port.To = port.Value
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
	} else {
		for _, port := range alloc.AllocatedResources.Shared.Ports {
			if port.To < 1 {
				port.To = port.Value
			}
			for _, proto := range []string{"tcp", "udp"} {
				portMapping := cni.PortMapping{
					HostPort:      int32(port.Value),
					ContainerPort: int32(port.To),
					Protocol:      proto,
				}
				if !ignoreHostIP {
					portMapping.HostIP = port.HostIP
				}
				ports = append(ports, portMapping)
			}
		}
	}
	return ports
}
