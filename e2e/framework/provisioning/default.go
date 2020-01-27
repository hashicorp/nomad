package provisioning

import (
	"fmt"
	"os"
	"testing"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/uuid"
	vapi "github.com/hashicorp/vault/api"
)

// DefaultProvisioner is a noop provisioner that builds clients from environment
// variables according to the respective client configuration
var DefaultProvisioner Provisioner = new(singleClusterProvisioner)

type singleClusterProvisioner struct{}

// SetupTestRun in the default case is a no-op.
func (p *singleClusterProvisioner) SetupTestRun(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {
	return &ClusterInfo{}, nil
}

// SetupTestSuite in the default case is a no-op.
func (p *singleClusterProvisioner) SetupTestSuite(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {
	return &ClusterInfo{}, nil
}

// SetupTestCase in the default case only creates new clients and embeds the
// TestCase name into the ClusterInfo handle.
func (p *singleClusterProvisioner) SetupTestCase(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {
	// Build ID based off given name
	info := &ClusterInfo{
		ID:   uuid.Generate()[:8],
		Name: opts.Name,
	}

	// Build Nomad api client
	nomadClient, err := napi.NewClient(napi.DefaultConfig())
	if err != nil {
		return nil, err
	}
	info.NomadClient = nomadClient

	if opts.ExpectConsul {
		consulClient, err := capi.NewClient(capi.DefaultConfig())
		if err != nil {
			return nil, fmt.Errorf("expected Consul: %v", err)
		}
		info.ConsulClient = consulClient
	}

	if len(os.Getenv(vapi.EnvVaultAddress)) != 0 {
		vaultClient, err := vapi.NewClient(vapi.DefaultConfig())
		if err != nil && opts.ExpectVault {
			return nil, err
		}
		info.VaultClient = vaultClient
	} else if opts.ExpectVault {
		return nil, fmt.Errorf("vault client expected but environment variable %s not set",
			vapi.EnvVaultAddress)
	}

	return info, err
}

// all TearDown* methods of the default provisioner leave the test environment in place

func (p *singleClusterProvisioner) TearDownTestCase(_ *testing.T, _ string) error  { return nil }
func (p *singleClusterProvisioner) TearDownTestSuite(_ *testing.T, _ string) error { return nil }
func (p *singleClusterProvisioner) TearDownTestRun(_ *testing.T, _ string) error   { return nil }
