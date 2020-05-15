package allocrunner

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cni "github.com/containerd/go-cni"
	cnilibrary "github.com/containernetworking/cni/libcni"
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
	cni     cni.CNI
	cniConf []byte

	rand   *rand.Rand
	logger log.Logger
}

func newCNINetworkConfigurator(logger log.Logger, cniPath, cniInterfacePrefix, cniConfDir, networkName string) (*cniNetworkConfigurator, error) {
	cniConf, err := loadCNIConf(cniConfDir, networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to load CNI config: %v", err)
	}

	return newCNINetworkConfiguratorWithConf(logger, cniPath, cniInterfacePrefix, cniConf)
}

func newCNINetworkConfiguratorWithConf(logger log.Logger, cniPath, cniInterfacePrefix string, cniConf []byte) (*cniNetworkConfigurator, error) {
	conf := &cniNetworkConfigurator{
		cniConf: cniConf,
		rand:    rand.New(rand.NewSource(time.Now().Unix())),
		logger:  logger,
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
func (c *cniNetworkConfigurator) Setup(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	if err := c.ensureCNIInitialized(); err != nil {
		return err
	}

	// Depending on the version of bridge cni plugin used, a known race could occure
	// where two alloc attempt to create the nomad bridge at the same time, resulting
	// in one of them to fail. This rety attempts to overcome those erroneous failures.
	const retry = 3
	var firstError error
	for attempt := 1; ; attempt++ {
		//TODO eventually returning the IP from the result would be nice to have in the alloc
		if _, err := c.cni.Setup(ctx, alloc.ID, spec.Path, cni.WithCapabilityPortMap(getPortMapping(alloc))); err != nil {
			c.logger.Warn("failed to configure network", "err", err, "attempt", attempt)
			switch attempt {
			case 1:
				firstError = err
			case retry:
				return fmt.Errorf("failed to configure network: %v", firstError)
			}

			// Sleep for 1 second + jitter
			time.Sleep(time.Second + (time.Duration(c.rand.Int63n(1000)) * time.Millisecond))
			continue
		}
		break
	}

	return nil

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

	return c.cni.Remove(ctx, alloc.ID, spec.Path, cni.WithCapabilityPortMap(getPortMapping(alloc)))
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
