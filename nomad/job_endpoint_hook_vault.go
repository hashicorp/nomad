package nomad

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	vapi "github.com/hashicorp/vault/api"
)

// jobVaultHook is an job registration admission controllver for Vault blocks.
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

	// Check entity aliases.
	err = h.validateEntityAliases(vaultBlocks, tokenSecret)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// validatePolicies returns an error if the job contains Vault blocks that
// require policies that the requirest token is not allowed to access.
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
	if helper.SliceStringContains(allowedPolicies, "root") {
		return nil
	}

	subset, offending := helper.SliceStringIsSubset(allowedPolicies, jobPolicies)
	if !subset {
		return fmt.Errorf("Vault token doesn't allow access to the following policies: %s",
			strings.Join(offending, ", "))
	}

	return nil
}

// validateEntityAliases returns an error if the job contains Vault blocks that
// use an entity alias that are not allowed to be used.
//
// In order to use entity aliases in a job, the following conditions must
// be met:
//   - the token used to submit the job and the Nomad server configuration
//     must have a role
//   - both roles must allow access to all entity aliases defined in the job
//
// If the Nomad server is configured with a default entity alias, it will
// use that for any Vault block that don't specify one, so:
//   - the token used to submit the job must be allowed to use the default
//     entity alias
//   - except if all Vault blocks in the job define an alias, since in this
//     case the server alias would not be used.
func (h jobVaultHook) validateEntityAliases(
	blocks map[string]map[string]*structs.Vault,
	token *vapi.Secret,
) error {

	// Assign the default entity alias from the server to any vault block with
	// no entity alias already set
	vconf := h.srv.config.VaultConfig
	if vconf.EntityAlias != "" {
		for _, task := range blocks {
			for _, v := range task {
				if v.EntityAlias == "" {
					v.EntityAlias = vconf.EntityAlias
				}
			}
		}
	}

	aliases := structs.VaultEntityAliasesSet(blocks)
	if len(aliases) == 0 {
		return nil
	}

	var tokenData structs.VaultTokenData
	if err := structs.DecodeVaultSecretData(token, &tokenData); err != nil {
		return fmt.Errorf("failed to parse Vault token data: %v", err)
	}

	// Check if user token allows requested entity aliases.
	if tokenData.Role == "" {
		return fmt.Errorf("jobs with Vault entity aliases require the Vault token to have a role")
	}
	if err := h.validateRole(tokenData.Role, aliases); err != nil {
		return fmt.Errorf("failed to validate entity alias against Vault token: %v", err)
	}

	// Check if Nomad server role allows requested entity aliases.
	if vconf.Role == "" {
		return fmt.Errorf("jobs with Vault entity aliases require the Nomad server to have a Vault role")
	}
	if err := h.validateRole(vconf.Role, aliases); err != nil {
		return fmt.Errorf("failed to validate entity alias against Nomad server configuration: %v", err)
	}

	return nil
}

// validateRole returns an error if the given role doesn't allow some of the
// aliases to be used.
func (h jobVaultHook) validateRole(role string, aliases []string) error {
	s, err := h.srv.vault.LookupTokenRole(context.Background(), role)
	if err != nil {
		return err
	}

	var data structs.VaultTokenRoleData
	if err := structs.DecodeVaultSecretData(s, &data); err != nil {
		return fmt.Errorf("failed to parse role data: %v", err)
	}

	invalidAliases := []string{}
	for _, a := range aliases {
		if !data.AllowsEntityAlias(a) {
			invalidAliases = append(invalidAliases, a)
		}
	}
	if len(invalidAliases) > 0 {
		return fmt.Errorf("role doesn't allow access to the following entity aliases: %s",
			strings.Join(invalidAliases, ", "))
	}
	return nil
}
