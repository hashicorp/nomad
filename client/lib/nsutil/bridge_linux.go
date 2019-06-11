package nsutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/libcni"
)

const (
	EnvCNIPath = "CNI_PATH"
)

type PortMapping struct {
	Host      int    `json:"hostPort"`
	Container int    `json:"containerPort"`
	Proto     string `json:"protocol"`
}

func SetupBridgeNetworking(allocID string, nsPath string, portMappings []*PortMapping) error {
	netconf, err := libcni.ConfListFromBytes([]byte(nomadCNIConfig))
	if err != nil {
		return err
	}
	containerID := fmt.Sprintf("nomad-%s", allocID[:8])
	cninet := libcni.NewCNIConfig(filepath.SplitList(os.Getenv(EnvCNIPath)), nil)

	rt := &libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       nsPath,
		IfName:      "eth0",
		CapabilityArgs: map[string]interface{}{
			"portMappings": portMappings,
		},
	}

	result, err := cninet.AddNetworkList(context.TODO(), netconf, rt)
	if result != nil {
		result.Print()
	}

	return err
}

func TeardownBridgeNetworking(allocID, nsPath string, portMappings []*PortMapping) error {
	netconf, err := libcni.ConfListFromBytes([]byte(nomadCNIConfig))
	if err != nil {
		return err
	}

	containerID := fmt.Sprintf("nomad-%s", allocID[:8])
	cninet := libcni.NewCNIConfig(filepath.SplitList(os.Getenv(EnvCNIPath)), nil)
	rt := &libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       nsPath,
		IfName:      "eth0",
		CapabilityArgs: map[string]interface{}{
			"portMappings": portMappings,
		},
	}
	err = cninet.DelNetworkList(context.TODO(), netconf, rt)

	return err
}

const nomadCNIConfig = `{
	"cniVersion": "0.4.0",
	"name": "nomad",
	"plugins": [
		{
			"type": "bridge",
			"bridge": "nomad",
			"isDefaultGateway": true,
			"ipMasq": true,
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{
							"subnet": "172.26.66.0/23"
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
