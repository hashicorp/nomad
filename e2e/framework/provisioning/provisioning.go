package provisioning

import (
	"testing"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	vapi "github.com/hashicorp/vault/api"
)

func NewProvisioner(config ProvisionerConfig) Provisioner {
	if config.IsLocal {
		return DefaultProvisioner
	}
	if config.VagrantBox != "" {
		return PreProvisioner(ProvisionerConfigVagrant(config))
	}
	if config.TerraformConfig != "" {
		return PreProvisioner(ProvisionerConfigTerraform(config))
	}
	return DefaultProvisioner
}

// ProvisionerConfig defines options for the entire lifecycle of the provisioner.
type ProvisionerConfig struct {
	IsLocal         bool
	VagrantBox      string
	TerraformConfig string

	NomadSha         string
	NomadVersion     string
	NomadLocalBinary string
}

// Provisioner interface is used by the test framework to provision Nomad.
// The Setup* methods should be used to create a Nomad cluster at the
// appropriate stage. The returned ClusterInfo handle helps TestCases
// isolate test state by using the ClusterInfo.ID as part of job IDs.
type Provisioner interface {
	// SetupTestRun is called at the start of the entire test run.
	SetupTestRun(t *testing.T, opts SetupOptions) (*ClusterInfo, error)

	// SetupTestSuite is called at the start of each TestSuite.
	// TODO: no current provisioner implementation uses this, but we
	// could use it to provide each TestSuite with an entirely separate
	// Nomad cluster.
	SetupTestSuite(t *testing.T, opts SetupOptions) (*ClusterInfo, error)

	// SetupTestCase is called at the start of each TestCase in every TestSuite.
	SetupTestCase(t *testing.T, opts SetupOptions) (*ClusterInfo, error)

	// TODO: no current provisioner implementation uses any of these,
	// but it's the obvious need if we setup/teardown after each TestSuite
	// or TestCase.

	// TearDownTestCase is called after each TestCase in every TestSuite.
	TearDownTestCase(t *testing.T, clusterID string) error

	// TearDownTestSuite is called after every TestSuite.
	TearDownTestSuite(t *testing.T, clusterID string) error

	// TearDownTestRun is called at the end of the entire test run.
	TearDownTestRun(t *testing.T, clusterID string) error
}

// SetupOptions defines options to be given to the Provisioner when
// calling Setup* methods.
type SetupOptions struct {
	Name         string
	ExpectConsul bool // If true, fails if a Consul client can't be configured
	ExpectVault  bool // If true, fails if a Vault client can't be configured
}

// ClusterInfo is a handle to a provisioned cluster, along with clients
// a test run can use to connect to the cluster.
type ClusterInfo struct {
	ID           string
	Name         string
	NomadClient  *napi.Client
	ConsulClient *capi.Client
	VaultClient  *vapi.Client
}
