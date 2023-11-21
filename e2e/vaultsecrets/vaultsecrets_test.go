// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultsecrets

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

const ns = ""

func TestVaultSecrets(t *testing.T) {
	// use a random suffix to encapsulate test keys, polices, etc.
	// for cleanup from vault
	testID := uuid.Generate()[0:8]
	secretsPath := "secrets-" + testID
	pkiPath := "pki-" + testID
	secretValue := uuid.Generate()
	secretKey := secretsPath + "/data/myapp"
	pkiCertIssue := pkiPath + "/issue/nomad"
	policyID := "access-secrets-" + testID
	wc := &e2e.WaitConfig{Retries: 500}
	interval, retries := wc.OrDefault()

	// configure KV secrets engine
	// Note: the secret key is written to 'secret-###/myapp' but the kv2 API
	// for Vault implicitly turns that into 'secret-###/data/myapp' so we
	// need to use the longer path for everything other than kv put/get
	e2e.MustCommand(t, "vault secrets enable -path=%s kv-v2", secretsPath)
	e2e.CleanupCommand(t, "vault secrets disable %s", secretsPath)
	e2e.MustCommand(t, "vault kv put %s/myapp key=%s", secretsPath, secretValue)
	e2e.MustCommand(t, "vault secrets tune -max-lease-ttl=1m %s", secretsPath)

	// configure PKI secrets engine
	e2e.MustCommand(t, "vault secrets enable -path=%s pki", pkiPath)
	e2e.CleanupCommand(t, "vault secrets disable %s", pkiPath)
	e2e.MustCommand(t, "vault write %s/root/generate/internal "+
		"common_name=service.consul ttl=1h", pkiPath)
	e2e.MustCommand(t, "vault write %s/roles/nomad "+
		"allowed_domains=service.consul "+
		"allow_subdomains=true "+
		"generate_lease=true "+
		"max_ttl=1m", pkiPath)
	e2e.MustCommand(t, "vault secrets tune -max-lease-ttl=1m %s", pkiPath)

	// we can't set an empty policy in our job, so write a bogus policy that
	// doesn't have access to any of the paths we're using
	writePolicy(t, policyID, "./input/policy-bad.hcl", testID)

	submission, cleanJob := jobs3.Submit(t,
		"./input/secrets.nomad",
		jobs3.Detach(),
		jobs3.ReplaceInJobSpec("TESTID", testID),
		jobs3.ReplaceInJobSpec("DEPLOYNUMBER", "FIRST"),
	)
	t.Cleanup(cleanJob)
	jobID := submission.JobID()
	// job doesn't have access to secrets, so they can't start
	err := e2e.WaitForAllocStatusExpected(jobID, ns, []string{"pending"})
	must.NoError(t, err, must.Sprint("expected pending allocation"))

	// we should get a task event about why they can't start
	expect := fmt.Sprintf("Missing: vault.read(%s), vault.write(%s", secretKey, pkiCertIssue)
	allocID := submission.AllocID("group")

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err := e2e.Command("nomad", "alloc", "status", allocID)
		must.NoError(t, err, must.Sprint("could not get allocation status"))
		return strings.Contains(out, expect),
			fmt.Errorf("expected '%s', got\n%v", expect, out)
	}, func(e error) {
		must.NoError(t, e)
	})

	// write a working policy and redeploy
	writePolicy(t, policyID, "./input/policy-good.hcl", testID)
	submission.Rerun(jobs3.ReplaceInJobSpec("FIRST", "SECOND"))

	// record the rough start of vault token TTL window, so that we don't have
	// to wait excessively later on
	ttlStart := time.Now()

	// job should be now unblocked
	err = e2e.WaitForAllocStatusExpected(jobID, ns, []string{"running", "complete"})
	must.NoError(t, err, must.Sprint("expected running->complete allocation"))

	allocID = submission.AllocID("group")

	renderedCert, err := waitForAllocSecret(allocID, "task", "/secrets/certificate.crt",
		func(out string) bool {
			return strings.Contains(out, "BEGIN CERTIFICATE")
		}, wc)
	must.NoError(t, err)

	_, err = waitForAllocSecret(allocID, "task", "/secrets/access.key",
		func(out string) bool {
			return strings.Contains(out, secretValue)
		}, wc)
	must.NoError(t, err)

	var re = regexp.MustCompile(`VAULT_TOKEN=(.*)`)

	// check vault token was written and save it for later comparison
	logs := submission.Exec("group", "task", []string{"env"})
	match := re.FindStringSubmatch(logs.Stdout)
	must.NotNil(t, match, must.Sprintf("could not find VAULT_TOKEN, got:%v\n", logs.Stdout))
	taskToken := match[1]

	// Update secret
	e2e.MustCommand(t, "vault kv put %s/myapp key=UPDATED", secretsPath)

	elapsed := time.Since(ttlStart)
	time.Sleep((time.Second * 60) - elapsed)

	// tokens will not be updated
	logs = submission.Exec("group", "task", []string{"env"})
	match = re.FindStringSubmatch(logs.Stdout)
	must.NotNil(t, match, must.Sprintf("could not find VAULT_TOKEN, got:%v\n", logs.Stdout))
	must.Eq(t, taskToken, match[1])

	// cert will be renewed
	_, err = waitForAllocSecret(allocID, "task", "/secrets/certificate.crt",
		func(out string) bool {
			return strings.Contains(out, "BEGIN CERTIFICATE") &&
				out != renderedCert
		}, wc)
	must.NoError(t, err)

	// secret will *not* be renewed because it doesn't have a lease to expire
	_, err = waitForAllocSecret(allocID, "task", "/secrets/access.key",
		func(out string) bool {
			return strings.Contains(out, secretValue)
		}, wc)
	must.NoError(t, err)

}

// We need to namespace the keys in the policy, so read it in and replace the
// values of the policy names
func writePolicy(t *testing.T, policyID, policyPath, testID string) {
	t.Helper()

	raw, err := os.ReadFile(policyPath)
	must.NoError(t, err)

	policyDoc := string(raw)
	policyDoc = strings.ReplaceAll(policyDoc, "TESTID", testID)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, "vault", "policy", "write", policyID, "-")
	stdin, err := cmd.StdinPipe()
	must.NoError(t, err)

	go func() {
		defer stdin.Close()
		_, err := io.WriteString(stdin, policyDoc)
		test.NoError(t, err)
	}()

	out, err := cmd.CombinedOutput()
	must.NoError(t, err, must.Sprintf("error writing policy, output: %s", out))
	e2e.CleanupCommand(t, "vault policy delete %s", policyID)
}

// waitForAllocSecret is similar to e2e.WaitForAllocFile but uses `alloc exec`
// to be able to read the secrets dir, which is not available to `alloc fs`
func waitForAllocSecret(allocID, taskID, path string, test func(string) bool, wc *e2e.WaitConfig) (string, error) {
	var err error
	var out string
	interval, retries := wc.OrDefault()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err = e2e.Command("nomad", "alloc", "exec", "-task", taskID, allocID, "cat", path)
		if err != nil {
			return false, fmt.Errorf("could not get file %q from allocation %q: %v",
				path, allocID, err)
		}
		return test(out),
			fmt.Errorf("test for file content failed: got\n%#v", out)
	}, func(e error) {
		err = e
	})
	return out, err
}
