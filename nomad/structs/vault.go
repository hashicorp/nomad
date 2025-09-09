// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
)

const (
	// VaultDefaultCluster is the name used for the Vault cluster that doesn't
	// have a name.
	VaultDefaultCluster = "default"
)

func ValidateVaultClusterName(cluster string) error {
	if !validConsulVaultClusterName.MatchString(cluster) {
		return fmt.Errorf("invalid name %q, must match regex %s", cluster, validConsulVaultClusterName)
	}

	return nil
}

// GetVaultClusterName gets the Vault cluster for this task. Only a single
// default cluster is supported in Nomad CE, but this function can be safely
// used for ENT as well because the appropriate Cluster value will be set at the
// time of job submission.
func (t *Task) GetVaultClusterName() string {
	if t.Vault != nil && t.Vault.Cluster != "" {
		return t.Vault.Cluster
	}
	return VaultDefaultCluster
}
