// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"regexp"
)

const (
	// ConsulDefaultCluster is the name used for the Consul cluster that doesn't
	// have a name.
	ConsulDefaultCluster = "default"

	// ConsulServiceIdentityNamePrefix is used in naming identities of consul
	// services
	ConsulServiceIdentityNamePrefix = "consul-service"

	// ConsulTaskIdentityNamePrefix is used in naming identities of consul tasks
	ConsulTaskIdentityNamePrefix = "consul"

	// ConsulWorkloadsDefaultAuthMethodName is the default JWT auth method name
	// that has to be configured in Consul in order to authenticate Nomad
	// services and tasks.
	ConsulWorkloadsDefaultAuthMethodName = "nomad-workloads"
)

// Consul represents optional per-group consul configuration.
type Consul struct {
	// Namespace in which to operate in Consul.
	Namespace string

	// Cluster (by name) to send API requests to
	Cluster string

	// Partition is the Consul admin partition where the workload should
	// run. Note that this should never be defaulted to "default" because
	// non-ENT Consul clusters don't have admin partitions
	Partition string
}

// Copy the Consul block.
func (c *Consul) Copy() *Consul {
	if c == nil {
		return nil
	}
	return &Consul{
		Namespace: c.Namespace,
		Cluster:   c.Cluster,
		Partition: c.Partition,
	}
}

// Equal returns whether c and o are the same.
func (c *Consul) Equal(o *Consul) bool {
	if c == nil || o == nil {
		return c == o
	}
	if c.Namespace != o.Namespace {
		return false
	}
	if c.Cluster != o.Cluster {
		return false
	}
	if c.Partition != o.Partition {
		return false
	}

	return true
}

// Validate returns whether c is valid.
func (c *Consul) Validate() error {
	// nothing to do here
	return nil
}

// IdentityName returns the name of the workload identity to be used to access
// this Consul cluster.
func (c *Consul) IdentityName() string {
	var clusterName string
	if c != nil && c.Cluster != "" {
		clusterName = c.Cluster
	} else {
		clusterName = ConsulDefaultCluster
	}

	return fmt.Sprintf("%s_%s", ConsulTaskIdentityNamePrefix, clusterName)
}

var (
	// validConsulVaultClusterName is the rule used to validate a Consul or
	// Vault cluster name.
	validConsulVaultClusterName = regexp.MustCompile("^[a-zA-Z0-9-_]{1,128}$")
)

func ValidateConsulClusterName(cluster string) error {
	if !validConsulVaultClusterName.MatchString(cluster) {
		return fmt.Errorf("invalid name %q, must match regex %s", cluster, validConsulVaultClusterName)
	}

	return nil
}
