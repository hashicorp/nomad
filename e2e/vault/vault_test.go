package vault

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	vapi "github.com/hashicorp/vault/api"
)

var integration = flag.Bool("integration", false, "run integration tests")

// harness is used to retrieve the required Vault test binaries
type harness struct {
	t      *testing.T
	binDir string
	os     string
	arch   string
}

// newHarness returns a new Vault test harness.
func newHarness(t *testing.T) *harness {
	return &harness{
		t:      t,
		binDir: filepath.Join(os.TempDir(), "vault-bins/"),
		os:     runtime.GOOS,
		arch:   runtime.GOARCH,
	}
}

// reconcile retrieves the desired binaries, returning a map of version to
// binary path
func (h *harness) reconcile() map[string]string {
	// Get the binaries we need to download
	missing := h.diff()

	// Create the directory for the binaries
	h.createBinDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	g, _ := errgroup.WithContext(ctx)
	for _, v := range missing {
		version := v
		g.Go(func() error {
			return h.get(version)
		})
	}
	if err := g.Wait(); err != nil {
		h.t.Fatalf("failed getting versions: %v", err)
	}

	binaries := make(map[string]string, len(versions))
	for _, v := range versions {
		binaries[v] = filepath.Join(h.binDir, v)
	}
	return binaries
}

// createBinDir creates the binary directory
func (h *harness) createBinDir() {
	// Check if the directory exists, otherwise create it
	f, err := os.Stat(h.binDir)
	if err != nil && !os.IsNotExist(err) {
		h.t.Fatalf("failed to stat directory: %v", err)
	}

	if f != nil && f.IsDir() {
		return
	} else if f != nil {
		if err := os.RemoveAll(h.binDir); err != nil {
			h.t.Fatalf("failed to remove file at directory path: %v", err)
		}
	}

	// Create the directory
	if err := os.Mkdir(h.binDir, 0700); err != nil {
		h.t.Fatalf("failed to make directory: %v", err)
	}
	if err := os.Chmod(h.binDir, 0700); err != nil {
		h.t.Fatalf("failed to chmod: %v", err)
	}
}

// diff returns the binaries that must be downloaded
func (h *harness) diff() (missing []string) {
	files, err := ioutil.ReadDir(h.binDir)
	if err != nil {
		if os.IsNotExist(err) {
			return versions
		}

		h.t.Fatalf("failed to stat directory: %v", err)
	}

	// Build the set we need
	missingSet := make(map[string]struct{}, len(versions))
	for _, v := range versions {
		missingSet[v] = struct{}{}
	}

	for _, f := range files {
		delete(missingSet, f.Name())
	}

	for k := range missingSet {
		missing = append(missing, k)
	}

	return missing
}

// get retrieves the given Vault binary
func (h *harness) get(version string) error {
	resp, err := http.Get(
		fmt.Sprintf("https://releases.hashicorp.com/vault/%s/vault_%s_%s_%s.zip",
			version, version, h.os, h.arch))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Wrap in an in-mem buffer
	b := bytes.NewBuffer(nil)
	io.Copy(b, resp.Body)
	resp.Body.Close()

	zreader, err := zip.NewReader(bytes.NewReader(b.Bytes()), resp.ContentLength)
	if err != nil {
		return err
	}

	if l := len(zreader.File); l != 1 {
		return fmt.Errorf("unexpected number of files in zip: %v", l)
	}

	// Copy the file to its destination
	file := filepath.Join(h.binDir, version)
	out, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0777)
	if err != nil {
		return err
	}
	defer out.Close()

	zfile, err := zreader.File[0].Open()
	if err != nil {
		return fmt.Errorf("failed to open zip file: %v", err)
	}

	if _, err := io.Copy(out, zfile); err != nil {
		return fmt.Errorf("failed to decompress file to destination: %v", err)
	}

	return nil
}

// TestVaultCompatibility tests compatibility across Vault versions
func TestVaultCompatibility(t *testing.T) {
	if !*integration {
		t.Skip("skipping test in non-integration mode.")
	}

	h := newHarness(t)
	vaultBinaries := h.reconcile()

	for version, vaultBin := range vaultBinaries {
		vbin := vaultBin
		t.Run(version, func(t *testing.T) {
			testVaultCompatibility(t, vbin)
		})
	}
}

// testVaultCompatibility tests compatibility with the given vault binary
func testVaultCompatibility(t *testing.T, vault string) {
	require := require.New(t)

	// Create a Vault server
	v := testutil.NewTestVaultFromPath(t, vault)
	defer v.Stop()

	token := setupVault(t, v.Client)

	// Create a Nomad agent using the created vault
	nomad := agent.NewTestAgent(t, t.Name(), func(c *agent.Config) {
		if c.Vault == nil {
			c.Vault = &config.VaultConfig{}
		}
		c.Vault.Enabled = helper.BoolToPtr(true)
		c.Vault.Token = token
		c.Vault.Role = "nomad-cluster"
		c.Vault.AllowUnauthenticated = helper.BoolToPtr(true)
		c.Vault.Addr = v.HTTPAddr
	})
	defer nomad.Shutdown()

	// Submit the Nomad job that requests a Vault token and cats that the Vault
	// token is there
	c := nomad.Client()
	j := c.Jobs()
	_, _, err := j.Register(job, nil)
	require.NoError(err)

	// Wait for there to be an allocation terminated successfully
	//var allocID string
	testutil.WaitForResult(func() (bool, error) {
		// Get the allocations for the job
		allocs, _, err := j.Allocations(*job.ID, false, nil)
		if err != nil {
			return false, err
		}
		l := len(allocs)
		switch l {
		case 0:
			return false, fmt.Errorf("want one alloc; got zero")
		case 1:
		default:
			// exit early
			t.Fatalf("too many allocations; something failed")
		}
		alloc := allocs[0]
		//allocID = alloc.ID
		if alloc.ClientStatus == "complete" {
			return true, nil
		}

		return false, fmt.Errorf("client status %q", alloc.ClientStatus)
	}, func(err error) {
		t.Fatalf("allocation did not finish: %v", err)
	})

}

// setupVault takes the Vault client and creates the required policies and
// roles. It returns the token that should be used by Nomad
func setupVault(t *testing.T, client *vapi.Client) string {
	// Write the policy
	sys := client.Sys()
	if err := sys.PutPolicy("nomad-server", policy); err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	// Build the role
	l := client.Logical()
	l.Write("auth/token/roles/nomad-cluster", role)

	// Create a new token with the role
	a := client.Auth().Token()
	req := vapi.TokenCreateRequest{
		Policies: []string{"nomad-server"},
		Period:   "72h",
		NoParent: true,
	}
	s, err := a.Create(&req)
	if err != nil {
		t.Fatalf("failed to create child token: %v", err)
	}

	// Get the client token
	if s == nil || s.Auth == nil {
		t.Fatalf("bad secret response: %+v", s)
	}

	return s.Auth.ClientToken
}
