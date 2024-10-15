// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultcompat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/go-version"
	goversion "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/api"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const (
	binDir     = "vault-bins"
	envGate    = "NOMAD_E2E_VAULTCOMPAT"
	envBaseDir = "NOMAD_E2E_VAULTCOMPAT_BASEDIR"
)

var (
	// minJWTVersion is the first version where the Nomad workload identity
	// auth flow is supported.
	//
	// 1.11.0 is when Vault added support for `user_claim_json_pointer`.
	// https://github.com/hashicorp/vault/pull/15593
	minJWTVersion = goversion.Must(goversion.NewVersion("1.11.0"))
)

func TestVaultCompat(t *testing.T) {
	if os.Getenv(envGate) != "1" {
		t.Skip(envGate + " is not set; skipping")
	}
	t.Run("testVaultVersions", testVaultVersions)
}

func testVaultVersions(t *testing.T) {
	versions := scanVaultVersions(t, getMinimumVersion(t))
	for b := range versions.Items() {
		downloadVaultBuild(t, b)
		testVaultBuild(t, b)
	}
}

func testVaultBuild(t *testing.T, b build) {
	version, err := goversion.NewVersion(b.Version)
	must.NoError(t, err)

	t.Run("vault("+b.Version+")", func(t *testing.T) {
		t.Run("legacy", func(t *testing.T) {
			testVaultLegacy(t, b)
		})

		if version.GreaterThanOrEqual(minJWTVersion) {
			t.Run("jwt", func(t *testing.T) {
				testVaultJWT(t, b)
			})
		}

		// give nomad and vault time to stop
		defer func() { time.Sleep(5 * time.Second) }()
	})
}

func validateLegacyAllocs(allocs []*nomadapi.AllocationListStub) error {
	if n := len(allocs); n != 1 {
		return fmt.Errorf("expected 1 alloc, got %d", n)
	}
	if s := allocs[0].ClientStatus; s != "complete" {
		return fmt.Errorf("expected alloc status complete, got %s", s)
	}
	return nil
}

func validateJWTAllocs(allocs []*nomadapi.AllocationListStub) error {
	if n := len(allocs); n != 2 {
		return fmt.Errorf("expected 2 allocs, got %d", n)
	}

	for _, alloc := range allocs {
		switch alloc.TaskGroup {

		// Verify all tasks in "success" group complete.
		case "success":
			if s := alloc.ClientStatus; s != "complete" {
				return fmt.Errorf("expected alloc status complete, got %s", s)
			}

		// Verify all tasks in "fail" group fail for the expected reasons.
		case "fail":
			for task, state := range alloc.TaskStates {
				switch task {

				// Verify "unauthorized" task can't access Vault secret.
				case "unauthorized":
					hasEvent := false
					for _, ev := range state.Events {
						if strings.Contains(ev.DisplayMessage, "Missing: vault.read") {
							hasEvent = true
							break
						}
					}
					if !hasEvent {
						got := make([]string, 0, len(state.Events))
						for _, ev := range state.Events {
							got = append(got, ev.DisplayMessage)
						}
						return fmt.Errorf("missing expected event, got [%v]", strings.Join(got, ", "))
					}

				// Verify "missing_vault" task fails.
				case "missing_vault":
					if !state.Failed {
						return fmt.Errorf("expected task to fail")
					}
				}
			}
		}
	}
	return nil
}

func runJob(t *testing.T, nc *nomadapi.Client, jobPath, ns string, validateAllocs func([]*nomadapi.AllocationListStub) error) {
	b, err := os.ReadFile(jobPath)
	must.NoError(t, err)

	jobs := nc.Jobs()
	job, err := jobs.ParseHCL(string(b), true)
	must.NoError(t, err, must.Sprint("failed to parse job HCL"))

	_, _, err = jobs.Register(job, nil)
	must.NoError(t, err, must.Sprint("failed to register job"))

	qOpts := &api.QueryOptions{Namespace: ns}
	wOpts := &api.WriteOptions{Namespace: ns}

	t.Cleanup(func() {
		jobs.Deregister(*job.Name, true, wOpts)
	})

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err := jobs.Allocations(*job.ID, false, qOpts)
			if err != nil {
				return err
			}
			return validateAllocs(allocs)
		}),
		wait.Timeout(20*time.Second),
		wait.Gap(1*time.Second),
	))

	t.Logf("success running job %s", *job.ID)
}

func startVault(t *testing.T, b build) (func(), *vaultapi.Client) {
	baseDir := os.Getenv(envBaseDir)
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	path := filepath.Join(baseDir, binDir, b.Version, "vault")
	vlt := testutil.NewTestVaultFromPath(t, path)
	return vlt.Stop, vlt.Client
}

func setupVaultLegacy(t *testing.T, vc *vaultapi.Client) {
	policy, err := os.ReadFile("input/policy_legacy.hcl")
	must.NoError(t, err)

	sys := vc.Sys()
	must.NoError(t, sys.PutPolicy("nomad-server", string(policy)))

	log := vc.Logical()
	log.Write("auth/token/roles/nomad-cluster", roleLegacy)

	token := vc.Auth().Token()
	secret, err := token.Create(&vaultapi.TokenCreateRequest{
		Policies: []string{"nomad-server"},
		Period:   "72h",
		NoParent: true,
	})
	must.NoError(t, err, must.Sprint("failed to create vault token"))
	must.NotNil(t, secret)
	must.NotNil(t, secret.Auth)
}

func setupVaultJWT(t *testing.T, vc *vaultapi.Client, jwksURL string) {
	logical := vc.Logical()
	sys := vc.Sys()

	// Enable JWT auth method and read back its accessor ID.
	err := sys.EnableAuthWithOptions(jwtPath, &vaultapi.MountInput{
		Type: "jwt",
	})
	must.NoError(t, err)

	secret, err := logical.Read(fmt.Sprintf("sys/auth/%s", jwtPath))
	must.NoError(t, err)
	must.NotNil(t, secret)

	jwtAuthAccessor := secret.Data["accessor"].(string)
	must.NotEq(t, "", jwtAuthAccessor)

	// Write JWT auth method config.
	_, err = logical.Write(fmt.Sprintf("auth/%s/config", jwtPath), authConfigJWT(jwksURL))
	must.NoError(t, err)

	// Write policies for general Nomad workloads and for restricted secrets.
	err = sys.PutPolicy("nomad-workloads", policyWID(jwtAuthAccessor))
	must.NoError(t, err)

	err = sys.PutPolicy("nomad-restricted", policyRestricted)
	must.NoError(t, err)

	// Write roles for each of the policies.
	rolePath := fmt.Sprintf("auth/%s/role/nomad-workloads", jwtPath)
	_, err = logical.Write(rolePath, roleWID([]string{"nomad-workloads"}))
	must.NoError(t, err)

	rolePath = fmt.Sprintf("auth/%s/role/nomad-restricted", jwtPath)
	_, err = logical.Write(rolePath, roleWID([]string{"nomad-restricted"}))
	must.NoError(t, err)

	entityOut, err := logical.Write("identity/entity", map[string]any{
		"name":     "default:restricted_jwt",
		"policies": []string{"nomad-restricted"},
	})
	must.NoError(t, err)
	entityID := entityOut.Data["id"]

	_, err = logical.Write("identity/entity-alias", map[string]any{
		"name":           "default:restricted_jwt",
		"canonical_id":   entityID,
		"mount_accessor": jwtAuthAccessor,
	})
	must.NoError(t, err)
}

func startNomad(t *testing.T, cb func(*testutil.TestServerConfig)) (func(), *nomadapi.Client) {
	bootstrapToken := uuid.Generate()
	ts := testutil.NewTestServer(t, func(c *testutil.TestServerConfig) {
		c.ACL.Enabled = true
		c.ACL.BootstrapToken = bootstrapToken
		c.DevMode = true
		c.Client = &testutil.ClientConfig{
			Enabled:      true,
			TotalCompute: 1000,
		}
		c.LogLevel = testlog.HCLoggerTestLevel().String()

		if cb != nil {
			cb(c)
		}
	})
	nc, err := nomadapi.NewClient(&nomadapi.Config{
		Address: "http://" + ts.HTTPAddr,
	})
	must.NoError(t, err, must.Sprint("unable to create nomad api client"))
	nc.SetSecretID(bootstrapToken)
	return ts.Stop, nc
}

func configureNomadVaultLegacy(vc *vaultapi.Client) func(*testutil.TestServerConfig) {
	return func(c *testutil.TestServerConfig) {
		c.Vaults = []*testutil.VaultConfig{{
			Enabled:              true,
			Address:              vc.Address(),
			Token:                vc.Token(),
			Role:                 "nomad-cluster",
			AllowUnauthenticated: pointer.Of(true),
		}}
	}
}

func configureNomadVaultJWT(vc *vaultapi.Client) func(*testutil.TestServerConfig) {
	return func(c *testutil.TestServerConfig) {
		c.Vaults = []*testutil.VaultConfig{{
			Enabled: true,
			// Server configs.
			DefaultIdentity: &testutil.WorkloadIdentityConfig{
				Audience: []string{"vault.io"},
				TTL:      "10m",
				ExtraClaims: map[string]string{
					"nomad_workload_id": "${job.namespace}:${job.id}",
				},
			},

			// Client configs.
			Address:            vc.Address(),
			JWTAuthBackendPath: jwtPath,
		}}
	}
}

func downloadVaultBuild(t *testing.T, b build) {
	baseDir := os.Getenv(envBaseDir)
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	path := filepath.Join(baseDir, binDir, b.Version)
	must.NoError(t, os.MkdirAll(path, 0755))

	if _, err := os.Stat(filepath.Join(path, "vault")); !os.IsNotExist(err) {
		t.Log("download: already have vault at", path)
		return
	}

	t.Log("download: installing vault at", path)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "hc-install", "install", "-version", b.Version, "-path", path, "vault")
	bs, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("download: failed to download %s, retrying once: %v", b.Version, err)
		cmd = exec.CommandContext(ctx, "hc-install", "install",
			"-version", b.Version, "-path", path, "vault")
		bs, err = cmd.CombinedOutput()
	}
	must.NoError(t, err, must.Sprintf("failed to download vault %s: %s", b.Version, string(bs)))
}

func getMinimumVersion(t *testing.T) *version.Version {
	v, err := version.NewVersion("1.11.0")
	must.NoError(t, err)
	return v
}

type build struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	URL     string `json:"url"`
}

func (b build) String() string { return b.Version }

func (b build) compare(o build) int {
	B := version.Must(version.NewVersion(b.Version))
	O := version.Must(version.NewVersion(o.Version))
	return B.Compare(O)
}

type vaultJSON struct {
	Versions map[string]struct {
		Builds []build `json:"builds"`
	}
}

func keep(b build) bool {
	switch {
	case b.OS != runtime.GOOS:
		return false
	case b.Arch != runtime.GOARCH:
		return false
	default:
		return true
	}
}

// A tracker keeps track of the set of patch versions for each minor version.
// The patch versions are stored in a treeset so we can grab the highest  patch
// version of each minor version at the end.
type tracker map[int]*set.TreeSet[build]

func (t tracker) add(v *version.Version, b build) {
	y := v.Segments()[1] // minor version

	// create the treeset for this minor version if needed
	if _, exists := t[y]; !exists {
		cmp := func(g, h build) int { return g.compare(h) }
		t[y] = set.NewTreeSet[build](cmp)
	}

	// insert the patch version into the set of patch versions for this minor version
	t[y].Insert(b)
}

func scanVaultVersions(t *testing.T, minimum *version.Version) *set.Set[build] {
	httpClient := cleanhttp.DefaultClient()
	httpClient.Timeout = 1 * time.Minute
	response, err := httpClient.Get("https://releases.hashicorp.com/vault/index.json")
	must.NoError(t, err, must.Sprint("unable to download vault versions index"))
	var payload vaultJSON
	must.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
	must.Close(t, response.Body)

	// sort the versions for the Y in each vault version X.Y.Z
	// this only works for vault 1.Y.Z which is fine for now
	track := make(tracker)

	for s, obj := range payload.Versions {
		v, err := version.NewVersion(s)
		must.NoError(t, err, must.Sprint("unable to parse vault version"))
		if !usable(v, minimum) {
			continue
		}
		for _, build := range obj.Builds {
			if keep(build) {
				track.add(v, build)
			}
		}
	}

	// take the latest patch version for each minor version
	result := set.New[build](len(track))
	for _, tree := range track {
		max := tree.Max()
		result.Insert(max)
	}
	return result
}
