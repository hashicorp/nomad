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
	"time"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
)

const ns = ""

type VaultSecretsTest struct {
	framework.TC
	secretsPath string
	pkiPath     string
	jobIDs      []string
	policies    []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "VaultSecrets",
		CanRunLocal: true,
		Consul:      true,
		Vault:       true,
		Cases: []framework.TestCase{
			new(VaultSecretsTest),
		},
	})
}

func (tc *VaultSecretsTest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *VaultSecretsTest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIDs {
		_, err := e2e.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err, "could not clean up job", id)
	}
	tc.jobIDs = []string{}

	for _, policy := range tc.policies {
		_, err := e2e.Command("vault", "policy", "delete", policy)
		f.Assert().NoError(err, "could not clean up vault policy", policy)
	}
	tc.policies = []string{}

	// disabling the secrets engines will wipe all the secrets as well
	_, err := e2e.Command("vault", "secrets", "disable", tc.secretsPath)
	f.Assert().NoError(err)
	_, err = e2e.Command("vault", "secrets", "disable", tc.pkiPath)
	f.Assert().NoError(err)

	_, err = e2e.Command("nomad", "system", "gc")
	f.NoError(err)
}

func (tc *VaultSecretsTest) TestVaultSecrets(f *framework.F) {

	// use a random suffix to encapsulate test keys, polices, etc.
	// for cleanup from vault
	testID := uuid.Generate()[0:8]
	jobID := "test-vault-secrets-" + testID
	tc.secretsPath = "secrets-" + testID
	tc.pkiPath = "pki-" + testID
	secretValue := uuid.Generate()
	secretKey := tc.secretsPath + "/data/myapp"
	pkiCertIssue := tc.pkiPath + "/issue/nomad"
	policyID := "access-secrets-" + testID
	index := 0
	wc := &e2e.WaitConfig{Retries: 500}
	interval, retries := wc.OrDefault()

	setupCmds := []string{

		// configure KV secrets engine
		// Note: the secret key is written to 'secret-###/myapp' but the kv2 API
		// for Vault implicitly turns that into 'secret-###/data/myapp' so we
		// need to use the longer path for everything other than kv put/get
		fmt.Sprintf("vault secrets enable -path=%s kv-v2", tc.secretsPath),
		fmt.Sprintf("vault kv put %s/myapp key=%s", tc.secretsPath, secretValue),
		fmt.Sprintf("vault secrets tune -max-lease-ttl=1m %s", tc.secretsPath),

		// configure PKI secrets engine
		fmt.Sprintf("vault secrets enable -path=%s pki", tc.pkiPath),
		fmt.Sprintf("vault write %s/root/generate/internal "+
			"common_name=service.consul ttl=1h", tc.pkiPath),
		fmt.Sprintf("vault write %s/roles/nomad "+
			"allowed_domains=service.consul "+
			"allow_subdomains=true "+
			"generate_lease=true "+
			"max_ttl=1m", tc.pkiPath),
		fmt.Sprintf("vault secrets tune -max-lease-ttl=1m %s", tc.pkiPath),
	}

	for _, setupCmd := range setupCmds {
		cmd := strings.Split(setupCmd, " ")
		out, err := e2e.Command(cmd[0], cmd[1:]...)
		f.NoError(err, fmt.Sprintf("error for %q:\n%s", setupCmd, out))
	}

	// we can't set an empty policy in our job, so write a bogus policy that
	// doesn't have access to any of the paths we're using
	out, err := writePolicy(policyID, "./vaultsecrets/input/policy-bad.hcl", testID)
	f.NoError(err, out)
	tc.policies = append(tc.policies, policyID)

	index++
	err = runJob(jobID, testID, index)
	f.NoError(err, "could not register job")
	tc.jobIDs = append(tc.jobIDs, jobID)

	// job doesn't have access to secrets, so they can't start
	err = e2e.WaitForAllocStatusExpected(jobID, ns, []string{"pending"})
	f.NoError(err, "expected pending allocation")

	// we should get a task event about why they can't start
	expect := fmt.Sprintf("Missing: vault.read(%s), vault.write(%s", secretKey, pkiCertIssue)

	allocID, err := latestAllocID(jobID)
	f.NoError(err)

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err := e2e.Command("nomad", "alloc", "status", allocID)
		f.NoError(err, "could not get allocation status")
		return strings.Contains(out, expect),
			fmt.Errorf("expected '%s', got\n%v", expect, out)
	}, func(e error) {
		f.NoError(e)
	})

	// write a working policy and redeploy
	out, err = writePolicy(policyID, "./vaultsecrets/input/policy-good.hcl", testID)
	f.NoError(err, out)
	index++
	err = runJob(jobID, testID, index)
	f.NoError(err, "could not register job")

	// record the rough start of vault token TTL window, so that we don't have
	// to wait excessively later on
	ttlStart := time.Now()

	// job should be now unblocked
	err = e2e.WaitForAllocStatusExpected(jobID, ns, []string{"running", "complete"})
	f.NoError(err, "expected running allocation")

	allocID, err = latestAllocID(jobID)
	f.NoError(err)

	renderedCert, err := waitForAllocSecret(allocID, "task", "/secrets/certificate.crt",
		func(out string) bool {
			return strings.Contains(out, "BEGIN CERTIFICATE")
		}, wc)
	f.NoError(err)

	_, err = waitForAllocSecret(allocID, "task", "/secrets/access.key",
		func(out string) bool {
			return strings.Contains(out, secretValue)
		}, wc)
	f.NoError(err)

	var re = regexp.MustCompile(`VAULT_TOKEN=(.*)`)

	// check vault token was written and save it for later comparison
	out, err = e2e.AllocExec(allocID, "task", "env", ns, nil)
	f.NoError(err)
	match := re.FindStringSubmatch(out)
	f.NotNil(match, fmt.Errorf("could not find VAULT_TOKEN, got:%v\n", out))
	taskToken := match[1]

	// Update secret
	out, err = e2e.Command("vault", "kv", "put",
		fmt.Sprintf("%s/myapp", tc.secretsPath), "key=UPDATED")
	f.NoError(err, out)

	elapsed := time.Since(ttlStart)
	time.Sleep((time.Second * 60) - elapsed)

	// tokens will not be updated
	out, err = e2e.AllocExec(allocID, "task", "env", ns, nil)
	f.NoError(err)
	match = re.FindStringSubmatch(out)
	f.NotNil(match, fmt.Errorf("could not find VAULT_TOKEN, got:%v\n", out))
	f.Equal(taskToken, match[1])

	// cert will be renewed
	_, err = waitForAllocSecret(allocID, "task", "/secrets/certificate.crt",
		func(out string) bool {
			return strings.Contains(out, "BEGIN CERTIFICATE") &&
				out != renderedCert
		}, wc)
	f.NoError(err)

	// secret will *not* be renewed because it doesn't have a lease to expire
	_, err = waitForAllocSecret(allocID, "task", "/secrets/access.key",
		func(out string) bool {
			return strings.Contains(out, secretValue)
		}, wc)
	f.NoError(err)

}

// We need to namespace the keys in the policy, so read it in and replace the
// values of the policy names
func writePolicy(policyID, policyPath, testID string) (string, error) {
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		return "", err
	}
	policyDoc := string(raw)
	policyDoc = strings.ReplaceAll(policyDoc, "TESTID", testID)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, "vault", "policy", "write", policyID, "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, policyDoc)
	}()

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// We need to namespace the vault paths in the job, so parse it
// and replace the values of the template and vault fields
func runJob(jobID, testID string, index int) error {

	raw, err := os.ReadFile("./vaultsecrets/input/secrets.nomad")
	if err != nil {
		return err
	}
	jobspec := string(raw)
	jobspec = strings.ReplaceAll(jobspec, "TESTID", testID)
	jobspec = strings.ReplaceAll(jobspec, "DEPLOYNUMBER", string(rune(index)))

	return e2e.RegisterFromJobspec(jobID, jobspec)
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

// this will always be sorted
func latestAllocID(jobID string) (string, error) {
	allocs, err := e2e.AllocsForJob(jobID, ns)
	if err != nil {
		return "", err
	}
	return allocs[0]["ID"], nil
}
