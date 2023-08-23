// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	vapi "github.com/hashicorp/vault/api"
)

// jobVaultHook is an job registration admission controller for Vault blocks.
type jobVaultHook struct {
	srv *Server
}

func (jobVaultHook) Name() string {
	return "vault"
}

func (h jobVaultHook) Validate(job *structs.Job) ([]error, error) {
	vaultBlocks := job.Vault()
	if len(vaultBlocks) == 0 {
		return nil, nil
	}

	vconf := h.srv.config.VaultConfig
	if !vconf.IsEnabled() {
		return nil, fmt.Errorf("Vault not enabled but used in the job")
	}

	// Return early if Vault configuration doesn't require authentication.
	if vconf.AllowsUnauthenticated() {
		return nil, nil
	}

	// At this point the job has a vault block and the server requires
	// authentication, so check if the user has the right permissions.
	if job.VaultToken == "" {
		return nil, fmt.Errorf("Vault used in the job but missing Vault token")
	}

	tokenSecret, err := h.srv.vault.LookupToken(context.Background(), job.VaultToken)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup Vault token: %v", err)
	}

	// Check namespaces.
	err = h.validateNamespaces(vaultBlocks, tokenSecret)
	if err != nil {
		return nil, err
	}

	// Check policies.
	err = h.validatePolicies(vaultBlocks, tokenSecret)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// validatePolicies returns an error if the job contains Vault blocks that
// require policies that the request token is not allowed to access.
func (jobVaultHook) validatePolicies(
	blocks map[string]map[string]*structs.Vault,
	token *vapi.Secret,
) error {

	jobPolicies := structs.VaultPoliciesSet(blocks)
	if len(jobPolicies) == 0 {
		return nil
	}

	allowedPolicies, err := token.TokenPolicies()
	if err != nil {
		return fmt.Errorf("failed to lookup Vault token policies: %v", err)
	}

	// If we are given a root token it can access all policies
	if slices.Contains(allowedPolicies, "root") {
		return nil
	}

	subset, offending := helper.IsSubset(allowedPolicies, jobPolicies)
	if !subset {
		return fmt.Errorf("Vault token doesn't allow access to the following policies: %s",
			strings.Join(offending, ", "))
	}

	return nil
}
