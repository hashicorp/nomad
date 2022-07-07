package e2eutil

import (
	"context"
	"fmt"
	"github.com/hashicorp/nomad/testutil"
	"io"
	"os/exec"
	"strings"
	"time"
)

// InitVaultForSecrets uses the testID throughout processing as a convention
// so that a well known value can be used throughout.
func InitVaultForSecrets(testID, maxTTL string) (string, error) {
	secretsPath := "secrets-" + testID
	pkiPath := "pki-" + testID

	setupCmds := []string{
		// configure KV secrets engine
		fmt.Sprintf("vault secrets enable -path=%s kv-v2", secretsPath),
		fmt.Sprintf("vault kv put %s/myapp key=%s", secretsPath, testID),
		fmt.Sprintf("vault secrets tune -max-lease-ttl=1m %s", secretsPath),

		// configure PKI secrets engine
		fmt.Sprintf("vault secrets enable -path=%s pki", pkiPath),
		fmt.Sprintf("vault write %s/root/generate/internal "+
			"common_name=service.consul ttl=1h", pkiPath),
		fmt.Sprintf("vault write %s/roles/nomad "+
			"allowed_domains=service.consul "+
			"allow_subdomains=true "+
			"generate_lease=true "+
			"max_ttl=%s", pkiPath, maxTTL),
		fmt.Sprintf("vault secrets tune -max-lease-ttl=%s %s", maxTTL, pkiPath),
	}

	for _, setupCmd := range setupCmds {
		cmd := strings.Split(setupCmd, " ")
		out, err := Command(cmd[0], cmd[1:]...)
		if err != nil {
			return out, err
		}
	}

	// write a working policy
	out, err := CreateVaultSecretsPolicy(testID)
	return out, err
}

func CreateVaultSecretsPolicy(testID string) (string, error) {
	policyID := "access-secrets-" + testID

	policyDoc := strings.ReplaceAll(policyTmpl, "TESTID", testID)

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

func DeleteVaultSecretsPolicy(testID string) (string, error) {
	policyID := "access-secrets-" + testID

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, "vault", "policy", "delete", policyID, "-")

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func WaitForVaultAllocSecret(allocID, taskID, path string, test func(string) bool, wc *WaitConfig) (string, error) {
	var err error
	var out string
	interval, retries := wc.OrDefault()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err = Command("nomad", "alloc", "exec", "-task", taskID, allocID, "cat", path)
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

var policyTmpl = `
path "secrets-TESTID/data/myapp" {
  capabilities = ["read"]
}

path "pki-TESTID/issue/nomad" {
  capabilities = ["create", "update", "read"]
}
`
