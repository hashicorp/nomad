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
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/go-version"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const (
	binDir  = "vault-bins"
	envGate = "NOMAD_E2E_VAULTCOMPAT"
)

func TestVaultCompat(t *testing.T) {
	if os.Getenv(envGate) != "1" {
		t.Skip(envGate + " is not set; skipping")
	}
	t.Run("testVaultVersions", testVaultVersions)
}

func testVaultVersions(t *testing.T) {
	versions := scanVaultVersions(t, getMinimumVersion(t))
	versions.ForEach(func(b build) bool {
		downloadVaultBuild(t, b)
		testVaultBuild(t, b)
		return true
	})
}

func testVaultBuild(t *testing.T, b build) {
	t.Run("vault("+b.Version+")", func(t *testing.T) {
		vStop, vc := startVault(t, b)
		defer vStop()
		setupVault(t, vc)

		nStop, nc := startNomad(t, vc)
		defer nStop()
		runCatJob(t, nc)

		// give nomad and vault time to stop
		defer func() { time.Sleep(5 * time.Second) }()
	})
}

func runCatJob(t *testing.T, nc *nomadapi.Client) {
	b, err := os.ReadFile("input/cat.hcl")
	must.NoError(t, err)

	jobs := nc.Jobs()
	job, err := jobs.ParseHCL(string(b), true)
	must.NoError(t, err, must.Sprint("failed to parse job HCL"))

	_, _, err = jobs.Register(job, nil)
	must.NoError(t, err, must.Sprint("failed to register job"))

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err := jobs.Allocations(*job.ID, false, nil)
			if err != nil {
				return err
			}
			if n := len(allocs); n != 1 {
				return fmt.Errorf("expected 1 alloc, got %d", n)
			}
			if s := allocs[0].ClientStatus; s != "complete" {
				return fmt.Errorf("expected alloc status complete, got %s", s)
			}
			return nil
		}),
		wait.Timeout(20*time.Second),
		wait.Gap(1*time.Second),
	))

	t.Log("success running cat job")

	_, _, err = jobs.Deregister(*job.Name, true, nil)
	must.NoError(t, err, must.Sprint("faild to deregister job"))
}

func startVault(t *testing.T, b build) (func(), *vaultapi.Client) {
	path := filepath.Join(os.TempDir(), binDir, b.Version, "vault")
	vlt := testutil.NewTestVaultFromPath(t, path)
	return vlt.Stop, vlt.Client
}

func setupVault(t *testing.T, vc *vaultapi.Client) {
	policy, err := os.ReadFile("input/policy.hcl")
	must.NoError(t, err)

	sys := vc.Sys()
	must.NoError(t, sys.PutPolicy("nomad-server", string(policy)))

	log := vc.Logical()
	log.Write("auth/token/roles/nomad-cluster", role)

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

func startNomad(t *testing.T, vc *vaultapi.Client) (func(), *nomadapi.Client) {
	ts := testutil.NewTestServer(t, func(c *testutil.TestServerConfig) {
		c.Vault = &testutil.VaultConfig{
			Enabled:              true,
			Address:              vc.Address(),
			Token:                vc.Token(),
			Role:                 "nomad-cluster",
			AllowUnauthenticated: true,
		}
		c.DevMode = true
		c.Client = &testutil.ClientConfig{
			Enabled:      true,
			TotalCompute: 1000,
		}
		c.LogLevel = testlog.HCLoggerTestLevel().String()
	})
	nc, err := nomadapi.NewClient(&nomadapi.Config{
		Address: "http://" + ts.HTTPAddr,
	})
	must.NoError(t, err, must.Sprint("unable to create nomad api client"))
	return ts.Stop, nc
}

func downloadVaultBuild(t *testing.T, b build) {
	path := filepath.Join(os.TempDir(), binDir, b.Version)
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
	must.NoError(t, err, must.Sprintf("failed to download vault %s: %s", b.Version, string(bs)))
}

func getMinimumVersion(t *testing.T) *version.Version {
	v, err := version.NewVersion("1.1.0")
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

func usable(v, minimum *version.Version) bool {
	switch {
	case v.Prerelease() != "":
		return false
	case v.Metadata() != "":
		return false
	case v.LessThan(minimum):
		return false
	default:
		return true
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
type tracker map[int]*set.TreeSet[build, set.Compare[build]]

func (t tracker) add(v *version.Version, b build) {
	y := v.Segments()[1] // minor version

	// create the treeset for this minor version if needed
	if _, exists := t[y]; !exists {
		cmp := func(g, h build) int { return g.compare(h) }
		t[y] = set.NewTreeSet[build, set.Compare[build]](cmp)
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
