// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secret

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/api"
	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

const ns = "default"

func TestVaultSecret(t *testing.T) {
	// Lookup the cluster ID which is the KV backend path start.
	clusterID, found := os.LookupEnv("CLUSTER_UNIQUE_IDENTIFIER")
	if !found {
		t.Fatal("CLUSTER_UNIQUE_IDENTIFIER env var not set")
	}

	// Generate our pathing for Vault and a secret value that we will check as
	// part of the test.
	secretCLIPath := filepath.Join(ns, "vault_secret", "testsecret")
	secretFullPath := filepath.Join(clusterID, "data", secretCLIPath)
	secretValue := uuid.Generate()

	// Create the secret at the correct mount point for this E2E cluster and use
	// the metadata delete command to permanently delete this when the test exits
	e2e.MustCommand(t, "vault kv put -mount=%s %s key=%s", clusterID, secretCLIPath, secretValue)
	e2e.CleanupCommand(t, "vault kv metadata delete -mount=%s %s", clusterID, secretCLIPath)

	submission, cleanJob := jobs3.Submit(t,
		"./input/vault_secret.hcl",
		jobs3.DisableRandomJobID(), // our path won't match the secret path with a random jobID
		jobs3.Namespace(ns),
		jobs3.Var("secret_path", secretFullPath),
	)
	t.Cleanup(cleanJob)

	// Validate the nomad variable was read and parsed into the expected
	// environment variable
	out := submission.Exec("group", "task", []string{"env"})
	must.StrContains(t, out.Stdout, fmt.Sprintf("TEST_SECRET=%s", secretValue))
}

func TestNomadSecret(t *testing.T) {
	// Generate our pathing for Vault and a secret value that we will check as
	// part of the test.
	secretFullPath := filepath.Join("nomad_secret", "testsecret")
	secretValue := uuid.Generate()

	nomadClient := e2e.NomadClient(t)

	opts := &api.WriteOptions{Namespace: ns}
	_, _, err := nomadClient.Variables().Create(&api.Variable{
		Namespace: ns,
		Path:      secretFullPath,
		Items:     map[string]string{"key": secretValue},
	}, opts)
	must.NoError(t, err)

	// create an ACL policy and attach it to the job ID this test will run
	myNamespacePolicy := api.ACLPolicy{
		Name:        "secret-block-policy",
		Rules:       fmt.Sprintf(`namespace "%s" {variables {path "*" {capabilities = ["read"]}}}`, ns),
		Description: "This namespace is for secrets block e2e testing",
		JobACL: &api.JobACL{
			Namespace: ns,
			JobID:     "nomad_secret",
		},
	}
	_, err = nomadClient.ACLPolicies().Upsert(&myNamespacePolicy, nil)
	must.NoError(t, err)

	t.Cleanup(func() {
		nomadClient.ACLPolicies().Delete("secret-block-policy", nil)
	})

	submission, cleanJob := jobs3.Submit(t,
		"./input/nomad_secret.hcl",
		jobs3.DisableRandomJobID(),
		jobs3.Namespace(ns),
		jobs3.Var("secret_path", secretFullPath),
	)
	t.Cleanup(cleanJob)

	// Validate the nomad variable was read and parsed into the expected
	// environment variable
	out := submission.Exec("group", "task", []string{"env"})
	must.StrContains(t, out.Stdout, fmt.Sprintf("TEST_SECRET=%s", secretValue))
}

func TestPluginSecret(t *testing.T) {
	// Generate a uuid value for the secret plugins env block which it will output
	// as a part of the result field.
	secretValue := uuid.Generate()

	submission, cleanJob := jobs3.Submit(t,
		"./input/custom_secret.hcl",
		jobs3.DisableRandomJobID(),
		jobs3.Namespace(ns),
		jobs3.Var("secret_value", secretValue),
	)
	t.Cleanup(cleanJob)

	// Validate the nomad variable was read and parsed into the expected
	// environment variable
	out := submission.Exec("group", "task", []string{"env"})
	must.StrContains(t, out.Stdout, fmt.Sprintf("TEST_SECRET=%s", secretValue))
}
