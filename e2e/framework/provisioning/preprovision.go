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

// PreProvisioner returns a provisioner that deploys Nomad to a cluster
// at the start of the whole test run and leaves it in place.
func PreProvisioner(targets *ProvisioningTargets) Provisioner {
	return &preProvisioner{
		Servers: targets.Servers,
		Clients: targets.Clients,
	}
}

type preProvisioner struct {
	Servers []*ProvisioningTarget // servers get provisioned before clients
	Clients []*ProvisioningTarget // leave empty for nodes that are both
}

// SetupTestRun deploys a Nomad cluster to the target environment. If the target
// environment has a Nomad cluster already, it will upgrade it to the desired
// version of leave it in place if it matches the ProvisioningTarget config.
func (p *preProvisioner) SetupTestRun(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {

	for _, server := range p.Servers {
		err := deploy(t, server)
		if err != nil {
			return nil, err
		}
	}

	for _, client := range p.Clients {
		err := deploy(t, client)
		if err != nil {
			return nil, err
		}
	}

	return &ClusterInfo{ID: "framework", Name: "framework"}, nil
}

// SetupTestSuite is a no-op.
func (p *preProvisioner) SetupTestSuite(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {
	return &ClusterInfo{ID: opts.Name, Name: opts.Name}, nil
}

// SetupTestCase in creates new clients and embeds the TestCase name into
// the ClusterInfo handle.
func (p *preProvisioner) SetupTestCase(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {
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
		if err != nil && opts.ExpectConsul {
			return nil, fmt.Errorf(
				"consul client expected but no Consul available: %v", err)
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
		return nil, fmt.Errorf(
			"vault client expected but Vault available: %v", err)
	}

	return info, err
}

// all TearDown* methods of preProvisioner leave the test environment in place

func (p *preProvisioner) TearDownTestCase(_ *testing.T, _ string) error  { return nil }
func (p *preProvisioner) TearDownTestSuite(_ *testing.T, _ string) error { return nil }
func (p *preProvisioner) TearDownTestRun(_ *testing.T, _ string) error   { return nil }
