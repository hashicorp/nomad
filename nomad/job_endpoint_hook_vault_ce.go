// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
	vapi "github.com/hashicorp/vault/api"
)

// validateNamespaces returns an error if the job contains multiple Vault
// namespaces.
func (jobVaultHook) validateNamespaces(
	blocks map[string]map[string]*structs.Vault,
	token *vapi.Secret,
) error {

	requestedNamespaces := structs.VaultNamespaceSet(blocks)
	if len(requestedNamespaces) > 0 {
		return fmt.Errorf("%w, Namespaces: %s", ErrMultipleNamespaces, strings.Join(requestedNamespaces, ", "))
	}
	return nil
}

func (h jobVaultHook) validateClustersForNamespace(_ *structs.Job, blocks map[string]map[string]*structs.Vault) error {
	for _, tg := range blocks {
		for _, vault := range tg {
			if vault.Cluster != "default" {
				return errors.New("non-default Vault cluster requires Nomad Enterprise")
			}
		}
	}

	return nil
}

func (j jobVaultHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			if task.Vault == nil || task.Vault.Cluster != "" {
				continue
			}
			task.Vault.Cluster = "default"
		}
	}

	return job, nil, nil
}
