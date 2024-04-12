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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	cni "github.com/containerd/go-cni"
	cnilibrary "github.com/containernetworking/cni/libcni"
	"github.com/coreos/go-iptables/iptables"
	consulIPTables "github.com/hashicorp/consul/sdk/iptables"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set/v2"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/envoy"
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
	nodeAttrs               map[string]string
	nodeMeta                map[string]string

	rand   *rand.Rand
	logger log.Logger
}

func newCNINetworkConfigurator(logger log.Logger, cniPath, cniInterfacePrefix, cniConfDir, networkName string, ignorePortMappingHostIP bool, node *structs.Node) (*cniNetworkConfigurator, error) {
	cniConf, err := loadCNIConf(cniConfDir, networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to load CNI config: %v", err)
	}

	return newCNINetworkConfiguratorWithConf(logger, cniPath, cniInterfacePrefix, ignorePortMappingHostIP, cniConf, node)
}

func newCNINetworkConfiguratorWithConf(logger log.Logger, cniPath, cniInterfacePrefix string, ignorePortMappingHostIP bool, cniConf []byte, node *structs.Node) (*cniNetworkConfigurator, error) {
	conf := &cniNetworkConfigurator{
		cniConf:                 cniConf,
		rand:                    rand.New(rand.NewSource(time.Now().Unix())),
		logger:                  logger,
		ignorePortMappingHostIP: ignorePortMappingHostIP,
		nodeAttrs:               node.Attributes,
		nodeMeta:                node.Meta,
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

const (
	ConsulIPTablesConfigEnvVar = "CONSUL_IPTABLES_CONFIG"
)

// Setup calls the CNI plugins with the add action
func (c *cniNetworkConfigurator) Setup(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) (*structs.AllocNetworkStatus, error) {
	if err := c.ensureCNIInitialized(); err != nil {
		return nil, err
	}
	cniArgs := map[string]string{
		// CNI plugins are called one after the other with the same set of
		// arguments. Passing IgnoreUnknown=true signals to plugins that they
		// should ignore any arguments they don't understand
		"IgnoreUnknown": "true",
	}

	portMaps := getPortMapping(alloc, c.ignorePortMappingHostIP)

	tproxyArgs, err := c.setupTransparentProxyArgs(alloc, spec, portMaps)
	if err != nil {
		return nil, err
	}
	if tproxyArgs != nil {
		iptablesCfg, err := json.Marshal(tproxyArgs)
		if err != nil {
			return nil, err
		}
		cniArgs[ConsulIPTablesConfigEnvVar] = string(iptablesCfg)
	}

	// Depending on the version of bridge cni plugin used, a known race could occure
	// where two alloc attempt to create the nomad bridge at the same time, resulting
	// in one of them to fail. This rety attempts to overcome those erroneous failures.
	const retry = 3
	var firstError error
	var res *cni.Result
	for attempt := 1; ; attempt++ {
		var err error
		if res, err = c.cni.Setup(ctx, alloc.ID, spec.Path,
			cni.WithCapabilityPortMap(portMaps.ports),
			cni.WithLabels(cniArgs), // "labels" turn into CNI_ARGS
		); err != nil {
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

	allocNet, err := c.cniToAllocNet(res)
	if err != nil {
		return nil, err
	}

	// overwrite the nameservers with Consul DNS, if we have it; we don't need
	// the port because the iptables rule redirects port 53 traffic to it
	if tproxyArgs != nil && tproxyArgs.ConsulDNSIP != "" {
		if allocNet.DNS == nil {
			allocNet.DNS = &structs.DNSConfig{
				Servers:  []string{},
				Searches: []string{},
				Options:  []string{},
			}
		}
		allocNet.DNS.Servers = []string{tproxyArgs.ConsulDNSIP}
	}

	return allocNet, nil
}

// setupTransparentProxyArgs returns a Consul SDK iptables configuration if the
// allocation has a transparent_proxy block
func (c *cniNetworkConfigurator) setupTransparentProxyArgs(alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec, portMaps *portMappings) (*consulIPTables.Config, error) {

	var tproxy *structs.ConsulTransparentProxy
	var cluster string
	var proxyUID string
	var proxyInboundPort int
	var proxyOutboundPort int

	var exposePorts []string
	outboundPorts := []string{}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, svc := range tg.Services {

		if svc.Connect.HasTransparentProxy() {

			tproxy = svc.Connect.SidecarService.Proxy.TransparentProxy
			cluster = svc.Cluster

			// The default value matches the Envoy UID. The cluster admin can
			// set this value to something non-default if they have a custom
			// Envoy container with a different UID
			proxyUID = c.nodeMeta[envoy.DefaultTransparentProxyUIDParam]
			if tproxy.UID != "" {
				proxyUID = tproxy.UID
			}

			// The value for the outbound Envoy port. The default value matches
			// the default TransparentProxy service default for
			// OutboundListenerPort. If the cluster admin sets this value to
			// something non-default, they'll need to update the metadata on all
			// the nodes to match. see also:
			// https://developer.hashicorp.com/consul/docs/connect/config-entries/service-defaults#transparentproxy
			if tproxy.OutboundPort != 0 {
				proxyOutboundPort = int(tproxy.OutboundPort)
			} else {
				outboundPortAttr := c.nodeMeta[envoy.DefaultTransparentProxyOutboundPortParam]
				parsedOutboundPort, err := strconv.ParseUint(outboundPortAttr, 10, 16)
				if err != nil {
					return nil, fmt.Errorf(
						"could not parse default_outbound_port %q as port number: %w",
						outboundPortAttr, err)
				}
				proxyOutboundPort = int(parsedOutboundPort)
			}

			// The inbound port is the service port exposed on the Envoy proxy
			envoyPortLabel := "connect-proxy-" + svc.Name
			if envoyPort, ok := portMaps.get(envoyPortLabel); ok {
				proxyInboundPort = int(envoyPort.HostPort)
			}

			// Extra user-defined ports that get excluded from outbound redirect
			if len(tproxy.ExcludeOutboundPorts) == 0 {
				outboundPorts = nil
			} else {
				outboundPorts = helper.ConvertSlice(tproxy.ExcludeOutboundPorts,
					func(p uint16) string { return fmt.Sprint(p) })
			}

			// The set of ports we'll exclude from inbound redirection
			exposePortSet := set.From(exposePorts)

			// We always expose reserved ports so that the allocation is
			// reachable from the outside world.
			for _, network := range tg.Networks {
				for _, port := range network.ReservedPorts {
					exposePortSet.Insert(fmt.Sprint(port.To))
				}
			}

			// ExcludeInboundPorts can be either a numeric port number or a port
			// label that we need to convert into a port number
			for _, portLabel := range tproxy.ExcludeInboundPorts {
				if _, err := strconv.ParseUint(portLabel, 10, 16); err == nil {
					exposePortSet.Insert(portLabel)
					continue
				}
				if port, ok := portMaps.get(portLabel); ok {
					exposePortSet.Insert(
						strconv.FormatInt(int64(port.ContainerPort), 10))
				}
			}

			// We also exclude Expose.Paths. Any health checks with expose=true
			// will have an Expose block added by the server, so this allows
			// health checks to work as expected without passing thru Envoy
			if svc.Connect.SidecarService.Proxy.Expose != nil {
				for _, path := range svc.Connect.SidecarService.Proxy.Expose.Paths {
					if port, ok := portMaps.get(path.ListenerPort); ok {
						exposePortSet.Insert(
							strconv.FormatInt(int64(port.ContainerPort), 10))
					}
				}
			}

			if exposePortSet.Size() > 0 {
				exposePorts = exposePortSet.Slice()
				slices.Sort(exposePorts)
			}

			// Only one Connect block is allowed with tproxy. This will have
			// been validated on job registration
			break
		}
	}

	if tproxy != nil {
		var dnsAddr string
		var dnsPort int
		if !tproxy.NoDNS {
			dnsAddr, dnsPort = c.dnsFromAttrs(cluster)
		}

		consulIPTablesCfgMap := &consulIPTables.Config{
			// Traffic in the DNSChain is directed to the Consul DNS Service IP.
			// For outbound TCP and UDP traffic going to port 53 (DNS), jump to
			// the DNSChain. Only redirect traffic that's going to consul's DNS
			// IP.
			ConsulDNSIP:   dnsAddr,
			ConsulDNSPort: dnsPort,

			// Don't redirect proxy traffic back to itself, return it to the
			// next chain for processing.
			ProxyUserID: proxyUID,

			// Redirects inbound TCP traffic hitting the PROXY_IN_REDIRECT chain
			// to Envoy's inbound listener port.
			ProxyInboundPort: proxyInboundPort,

			// Redirects outbound TCP traffic hitting PROXY_REDIRECT chain to
			// Envoy's outbound listener port.
			ProxyOutboundPort: proxyOutboundPort,

			ExcludeInboundPorts:  exposePorts,
			ExcludeOutboundPorts: outboundPorts,
			ExcludeOutboundCIDRs: tproxy.ExcludeOutboundCIDRs,
			ExcludeUIDs:          tproxy.ExcludeUIDs,
			NetNS:                spec.Path,
		}

		return consulIPTablesCfgMap, nil
	}

	return nil, nil
}

func (c *cniNetworkConfigurator) dnsFromAttrs(cluster string) (string, int) {
	var dnsAddrAttr, dnsPortAttr string
	if cluster == structs.ConsulDefaultCluster || cluster == "" {
		dnsAddrAttr = "consul.dns.addr"
		dnsPortAttr = "consul.dns.port"
	} else {
		dnsAddrAttr = "consul." + cluster + ".dns.addr"
		dnsPortAttr = "consul." + cluster + ".dns.port"
	}

	dnsAddr, ok := c.nodeAttrs[dnsAddrAttr]
	if !ok || dnsAddr == "" {
		return "", 0
	}
	dnsPort, ok := c.nodeAttrs[dnsPortAttr]
	if !ok || dnsPort == "0" || dnsPort == "-1" {
		return "", 0
	}
	port, err := strconv.ParseUint(dnsPort, 10, 16)
	if err != nil {
		return "", 0 // note: this will have been checked in fingerprint
	}
	return dnsAddr, int(port)
}

// cniToAllocNet converts a cni.Result to an AllocNetworkStatus or returns an
// error. The first interface and IP with a sandbox and address set are
// preferred. Failing that the first interface with an IP is selected.
func (c *cniNetworkConfigurator) cniToAllocNet(res *cni.Result) (*structs.AllocNetworkStatus, error) {
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

	// Use the first DNS results, if non-empty
	if len(res.DNS) > 0 {
		cniDNS := res.DNS[0]
		if len(cniDNS.Nameservers) > 0 {
			netStatus.DNS = &structs.DNSConfig{
				Servers:  cniDNS.Nameservers,
				Searches: cniDNS.Search,
				Options:  cniDNS.Options,
			}
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

	portMap := getPortMapping(alloc, c.ignorePortMappingHostIP)

	if err := c.cni.Remove(ctx, alloc.ID, spec.Path, cni.WithCapabilityPortMap(portMap.ports)); err != nil {
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

// portMappings is a wrapper around a slice of cni.PortMapping that lets us
// index via the port's label, which isn't otherwise included in the
// cni.PortMapping struct
type portMappings struct {
	ports  []cni.PortMapping
	labels map[string]int // Label -> index into ports field
}

func (pm *portMappings) set(label string, port cni.PortMapping) {
	pm.ports = append(pm.ports, port)
	pm.labels[label] = len(pm.ports) - 1
}

func (pm *portMappings) get(label string) (cni.PortMapping, bool) {
	idx, ok := pm.labels[label]
	if !ok {
		return cni.PortMapping{}, false
	}
	return pm.ports[idx], true
}

// getPortMapping builds a list of cni.PortMapping structs that are used as the
// portmapping capability arguments for the portmap CNI plugin
func getPortMapping(alloc *structs.Allocation, ignoreHostIP bool) *portMappings {
	mappings := &portMappings{
		ports:  []cni.PortMapping{},
		labels: map[string]int{},
	}

	if len(alloc.AllocatedResources.Shared.Ports) == 0 && len(alloc.AllocatedResources.Shared.Networks) > 0 {
		for _, network := range alloc.AllocatedResources.Shared.Networks {
			for _, port := range append(network.DynamicPorts, network.ReservedPorts...) {
				if port.To < 1 {
					port.To = port.Value
				}
				for _, proto := range []string{"tcp", "udp"} {
					portMapping := cni.PortMapping{
						HostPort:      int32(port.Value),
						ContainerPort: int32(port.To),
						Protocol:      proto,
					}
					mappings.set(port.Label, portMapping)
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
				mappings.set(port.Label, portMapping)
			}
		}
	}
	return mappings
}
