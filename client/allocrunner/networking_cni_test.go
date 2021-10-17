//go:build linux
// +build linux

package allocrunner

import (
	"net"
	"testing"

	cni "github.com/containerd/go-cni"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCNI_cniToAllocNet_Fallback asserts if a CNI plugin result lacks an IP on
// its sandbox interface, the first IP found is used.
func TestCNI_cniToAllocNet_Fallback(t *testing.T) {
	// Calico's CNI plugin v3.12.3 has been observed to return the
	// following:
	cniResult := &cni.CNIResult{
		Interfaces: map[string]*cni.Config{
			"cali39179aa3-74": {},
			"eth0": {
				IPConfigs: []*cni.IPConfig{
					{
						IP: net.IPv4(192, 168, 135, 232),
					},
				},
			},
		},
	}

	// Only need a logger
	c := &cniNetworkConfigurator{
		logger: testlog.HCLogger(t),
	}
	allocNet, err := c.cniToAllocNet(cniResult)
	require.NoError(t, err)
	require.NotNil(t, allocNet)
	assert.Equal(t, "192.168.135.232", allocNet.Address)
	assert.Equal(t, "eth0", allocNet.InterfaceName)
	assert.Nil(t, allocNet.DNS)
}

// TestCNI_cniToAllocNet_Invalid asserts an error is returned if a CNI plugin
// result lacks any IP addresses. This has not been observed, but Nomad still
// must guard against invalid results from external plugins.
func TestCNI_cniToAllocNet_Invalid(t *testing.T) {
	cniResult := &cni.CNIResult{
		Interfaces: map[string]*cni.Config{
			"eth0": {},
			"veth1": {
				IPConfigs: []*cni.IPConfig{},
			},
		},
	}

	// Only need a logger
	c := &cniNetworkConfigurator{
		logger: testlog.HCLogger(t),
	}
	allocNet, err := c.cniToAllocNet(cniResult)
	require.Error(t, err)
	require.Nil(t, allocNet)
}
