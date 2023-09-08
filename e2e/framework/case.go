// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package framework

import (
	"fmt"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
)

// TestSuite defines a set of test cases and under what conditions to run them
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
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
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
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
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type TC struct {
	cluster *ClusterInfo
}

// Nomad returns a configured nomad api client
func (tc *TC) Nomad() *api.Client {
	return tc.cluster.NomadClient
}

// Consul returns a configured consul api client
func (tc *TC) Consul() *capi.Client {
	return tc.cluster.ConsulClient
}

// Name returns the name of the test case which is set to the name of the
// implementing type.
func (tc *TC) Name() string {
	return tc.cluster.Name
}

func (tc *TC) setClusterInfo(info *ClusterInfo) {
	tc.cluster = info
}
