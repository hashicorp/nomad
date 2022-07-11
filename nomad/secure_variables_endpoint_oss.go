//go:build !ent
// +build !ent

package nomad

import "github.com/hashicorp/nomad/nomad/structs"

func (sv *SecureVariables) enforceQuota(uArgs structs.SecureVariablesEncryptedUpsertRequest) error {
	return nil
}
