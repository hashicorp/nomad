package vault

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
)

const (
	// policy is the recommended Nomad Vault policy
	policy = `path "auth/token/create/nomad-cluster" {
  capabilities = ["update"]
}
path "auth/token/roles/nomad-cluster" {
  capabilities = ["read"]
}
path "auth/token/lookup-self" {
  capabilities = ["read"]
}

path "auth/token/lookup" {
  capabilities = ["update"]
}
path "auth/token/revoke-accessor" {
  capabilities = ["update"]
}
path "sys/capabilities-self" {
  capabilities = ["update"]
}
path "auth/token/renew-self" {
  capabilities = ["update"]
}`
)

var (
	// role is the recommended nomad cluster role
	role = map[string]interface{}{
		"disallowed_policies": "nomad-server",
		"explicit_max_ttl":    0,
		"name":                "nomad-cluster",
		"orphan":              false,
		"period":              259200,
		"renewable":           true,
	}

	// job is a test job that is used to request a Vault token and cat the token
	// out before exiting.
	job = &api.Job{
		ID:          helper.StringToPtr("test"),
		Type:        helper.StringToPtr("batch"),
		Datacenters: []string{"dc1"},
		TaskGroups: []*api.TaskGroup{
			{
				Name: helper.StringToPtr("test"),
				Tasks: []*api.Task{
					{
						Name:   "test",
						Driver: "raw_exec",
						Config: map[string]interface{}{
							"command": "cat",
							"args":    []string{"${NOMAD_SECRETS_DIR}/vault_token"},
						},
						Vault: &api.Vault{
							Policies: []string{"default"},
						},
					},
				},
				RestartPolicy: &api.RestartPolicy{
					Attempts: helper.IntToPtr(0),
					Mode:     helper.StringToPtr("fail"),
				},
			},
		},
	}
)
