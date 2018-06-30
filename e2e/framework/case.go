package framework

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSuite defines a set of test cases and under what conditions to run them
type TestSuite struct {
	Component string // Name of the component/system/feature tested

	CanRunLocal bool        // Flags if the cases are safe to run on a local nomad cluster
	Cases       []TestCase  // Cases to run
	Constraints Constraints // Environment constraints to follow
	Parallel    bool        // If true, will run test cases in parallel
	Slow        bool        // Slow test suites don't run by default

	// API Clients
	Consul bool
	Vault  bool
}

// Constraints that must be satisfied for a TestSuite to run
type Constraints struct {
	Provider    string   // Cloud provider ex. 'aws', 'azure', 'gcp'
	OS          string   // Operating system ex. 'windows', 'linux'
	Arch        string   // CPU architecture ex. 'amd64', 'arm64'
	Environment string   // Environment name ex. 'simple'
	Tags        []string // Generic tags that must all exist in the environment
}

func (c Constraints) matches(env Environment) error {
	if len(c.Provider) != 0 && c.Provider != env.Provider {
		return fmt.Errorf("provider constraint does not match environment")
	}

	if len(c.OS) != 0 && c.OS != env.OS {
		return fmt.Errorf("os constraint does not match environment")
	}

	if len(c.Arch) != 0 && c.Arch != env.Arch {
		return fmt.Errorf("arch constraint does not match environment")
	}

	if len(c.Environment) != 0 && c.Environment != env.Name {
		return fmt.Errorf("environment constraint does not match environment name")
	}

	for _, t := range c.Tags {
		if _, ok := env.Tags[t]; !ok {
			return fmt.Errorf("tags constraint failed, tag '%s' is not included in environment", t)
		}
	}
	return nil
}

// TC is the base test case which should be embedded in TestCase implementations.
// It also embeds testify assertions configured with the current *testing.T
// context. For more information on assertions:
// https://godoc.org/github.com/stretchr/testify/assert#Assertions
type TC struct {
	*assert.Assertions
	require *require.Assertions
	t       *testing.T

	cluster *ClusterInfo
	prefix  string
	name    string
}

// Nomad returns a configured nomad api client
func (tc *TC) Nomad() *api.Client {
	return tc.cluster.NomadClient
}

// Prefix will return a test case unique prefix which can be used to scope resources
// during parallel tests.
func (tc *TC) Prefix() string {
	return fmt.Sprintf("%s-", tc.cluster.ID)
}

// Name returns the name of the test case which is set to the name of the
// implementing type.
func (tc *TC) Name() string {
	return tc.cluster.Name
}

// T retrieves the current *testing.T context
func (tc *TC) T() *testing.T {
	return tc.t
}

// SetT sets the current *testing.T context
func (tc *TC) SetT(t *testing.T) {
	tc.t = t
	tc.Assertions = assert.New(t)
	tc.require = require.New(t)
}

// Require fetches a require flavor of testify assertions
// https://godoc.org/github.com/stretchr/testify/require
func (tc *TC) Require() *require.Assertions {
	return tc.require
}

func (tc *TC) setClusterInfo(info *ClusterInfo) {
	tc.cluster = info
}
