// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package framework

import (
	"fmt"
	"os"
	"testing"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/helper/uuid"
	vapi "github.com/hashicorp/vault/api"
)

// ClusterInfo is a handle to a provisioned cluster, along with clients
// a test run can use to connect to the cluster.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type ClusterInfo struct {
	ID           string
	Name         string
	NomadClient  *napi.Client
	ConsulClient *capi.Client
	VaultClient  *vapi.Client
}

// SetupOptions defines options to be given to the Provisioner when
// calling Setup* methods.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type SetupOptions struct {
	Name         string
	ExpectConsul bool // If true, fails if a Consul client can't be configured
	ExpectVault  bool // If true, fails if a Vault client can't be configured
}

// Provisioner interface is used by the test framework to provision API
// clients for a Nomad cluster, with the possibility of extending to provision
// standalone clusters for each test case in the future.
//
// The Setup* methods are hooks that get run at the appropriate stage. They
// return a ClusterInfo handle that helps TestCases isolate test state if
// they use the ClusterInfo.ID as part of job IDs.
//
// The TearDown* methods are hooks to clean up provisioned cluster state
// that isn't covered by the test case's implementation of AfterEachTest.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
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

// DefaultProvisioner is a Provisioner that doesn't deploy a Nomad cluster
// (because that's handled by Terraform elsewhere), but build clients from
// environment variables.
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
var DefaultProvisioner Provisioner = new(singleClusterProvisioner)

type singleClusterProvisioner struct{}

// SetupTestRun in the default case is a no-op.
func (p *singleClusterProvisioner) SetupTestRun(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {
	return &ClusterInfo{ID: "framework", Name: "framework"}, nil
}

// SetupTestSuite in the default case is a no-op.
func (p *singleClusterProvisioner) SetupTestSuite(t *testing.T, opts SetupOptions) (*ClusterInfo, error) {
	return &ClusterInfo{
		ID:   uuid.Generate()[:8],
		Name: opts.Name,
	}, nil
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
		useragent.SetHeaders(vaultClient)
		info.VaultClient = vaultClient
	} else if opts.ExpectVault {
		return nil, fmt.Errorf("vault client expected but environment variable %s not set",
			vapi.EnvVaultAddress)
	}

	return info, err
}

// all TearDown* methods of the default provisioner leave the test environment in place

// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func (p *singleClusterProvisioner) TearDownTestCase(_ *testing.T, _ string) error { return nil }

// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func (p *singleClusterProvisioner) TearDownTestSuite(_ *testing.T, _ string) error { return nil }

// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func (p *singleClusterProvisioner) TearDownTestRun(_ *testing.T, _ string) error { return nil }
