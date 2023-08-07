// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vaultcompat

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	vapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

var (
	integration = flag.Bool("integration", false, "run integration tests")
	minVaultVer = version.Must(version.NewVersion("0.14.1"))
)

// syncVault discovers available versions of Vault, downloads the binaries,
// returns a map of version to binary path as well as a sorted list of
// versions.
func syncVault(t *testing.T) ([]*version.Version, map[string]string) {

	binDir := filepath.Join(os.TempDir(), "vault-bins/")

	urls := vaultVersions(t)

	sorted, versions, err := pruneVersions(urls)
	require.NoError(t, err)

	// Get the binaries we need to download
	missing, err := missingVault(binDir, versions)
	require.NoError(t, err)

	// Create the directory for the binaries
	require.NoError(t, createBinDir(binDir))

	// Download in parallel
	start := time.Now()
	errCh := make(chan error, len(missing))
	for ver, url := range missing {
		go func(dst, url string) {
			errCh <- getVault(dst, url)
		}(filepath.Join(binDir, ver), url)
	}
	for i := 0; i < len(missing); i++ {
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Minute):
			require.Fail(t, "timed out downloading Vault binaries")
		}
	}
	if n := len(missing); n > 0 {
		t.Logf("Downloaded %d versions of Vault in %s", n, time.Now().Sub(start))
	}

	binaries := make(map[string]string, len(versions))
	for ver := range versions {
		binaries[ver] = filepath.Join(binDir, ver)
	}
	return sorted, binaries
}

// vaultVersions discovers available Vault versions from releases.hashicorp.com
// and returns a map of version to url.
func vaultVersions(t *testing.T) map[string]string {
	resp, err := http.Get("https://releases.hashicorp.com/vault/index.json")
	require.NoError(t, err)

	respJson := struct {
		Versions map[string]struct {
			Builds []struct {
				Version string `json:"version"`
				Os      string `json:"os"`
				Arch    string `json:"arch"`
				URL     string `json:"url"`
			} `json:"builds"`
		}
	}{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respJson))
	require.NoError(t, resp.Body.Close())

	versions := map[string]string{}
	for vk, vv := range respJson.Versions {
		gover, err := version.NewVersion(vk)
		if err != nil {
			t.Logf("error parsing Vault version %q -> %v", vk, err)
			continue
		}

		// Skip ancient versions
		if gover.LessThan(minVaultVer) {
			continue
		}

		// Skip prerelease and enterprise versions
		if gover.Prerelease() != "" || gover.Metadata() != "" {
			continue
		}

		url := ""
		for _, b := range vv.Builds {
			buildver, err := version.NewVersion(b.Version)
			if err != nil {
				t.Logf("error parsing Vault build version %q -> %v", b.Version, err)
				continue
			}

			if buildver.Prerelease() != "" {
				continue
			}

			if buildver.Metadata() != "" {
				continue
			}

			if b.Os != runtime.GOOS {
				continue
			}

			if b.Arch != runtime.GOARCH {
				continue
			}

			// Match!
			url = b.URL
			break
		}

		if url != "" {
			versions[vk] = url
		}
	}

	return versions
}

// pruneVersions only takes the latest Z for each X.Y.Z release. Returns a
// sorted list and map of kept versions.
func pruneVersions(all map[string]string) ([]*version.Version, map[string]string, error) {
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("0 Vault versions")
	}

	sorted := make([]*version.Version, 0, len(all))

	for k := range all {
		sorted = append(sorted, version.Must(version.NewVersion(k)))
	}

	sort.Sort(version.Collection(sorted))

	keep := make([]*version.Version, 0, len(all))

	for _, v := range sorted {
		segments := v.Segments()
		if len(segments) < 3 {
			// Drop malformed versions
			continue
		}

		if len(keep) == 0 {
			keep = append(keep, v)
			continue
		}

		last := keep[len(keep)-1].Segments()

		if segments[0] == last[0] && segments[1] == last[1] {
			// current X.Y == last X.Y, replace last with current
			keep[len(keep)-1] = v
		} else {
			// current X.Y != last X.Y, append
			keep = append(keep, v)
		}
	}

	// Create a new map of canonicalized versions to urls
	urls := make(map[string]string, len(keep))
	for _, v := range keep {
		origURL := all[v.Original()]
		if origURL == "" {
			return nil, nil, fmt.Errorf("missing version %s", v.Original())
		}
		urls[v.String()] = origURL
	}

	return keep, urls, nil
}

// createBinDir creates the binary directory
func createBinDir(binDir string) error {
	// Check if the directory exists, otherwise create it
	f, err := os.Stat(binDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat directory: %v", err)
	}

	if f != nil && f.IsDir() {
		return nil
	} else if f != nil {
		if err := os.RemoveAll(binDir); err != nil {
			return fmt.Errorf("failed to remove file at directory path: %v", err)
		}
	}

	// Create the directory
	if err := os.Mkdir(binDir, 075); err != nil {
		return fmt.Errorf("failed to make directory: %v", err)
	}
	if err := os.Chmod(binDir, 0755); err != nil {
		return fmt.Errorf("failed to chmod: %v", err)
	}

	return nil
}

// missingVault returns the binaries that must be downloaded. versions key must
// be the Vault version.
func missingVault(binDir string, versions map[string]string) (map[string]string, error) {
	files, err := os.ReadDir(binDir)
	if err != nil {
		if os.IsNotExist(err) {
			return versions, nil
		}

		return nil, fmt.Errorf("failed to stat directory: %v", err)
	}

	// Copy versions so we don't mutate it
	missingSet := make(map[string]string, len(versions))
	for k, v := range versions {
		missingSet[k] = v
	}

	for _, f := range files {
		delete(missingSet, f.Name())
	}

	return missingSet, nil
}

// getVault downloads the given Vault binary
func getVault(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Wrap in an in-mem buffer
	b := bytes.NewBuffer(nil)
	if _, err := io.Copy(b, resp.Body); err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}
	resp.Body.Close()

	zreader, err := zip.NewReader(bytes.NewReader(b.Bytes()), resp.ContentLength)
	if err != nil {
		return err
	}

	if l := len(zreader.File); l != 1 {
		return fmt.Errorf("unexpected number of files in zip: %v", l)
	}

	// Copy the file to its destination
	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0777)
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
		t.Skip("skipping test in non-integration mode: add -integration flag to run")
	}

	sorted, vaultBinaries := syncVault(t)

	for _, v := range sorted {
		ver := v.String()
		bin := vaultBinaries[ver]
		require.NotZerof(t, bin, "missing version: %s", ver)
		t.Run(ver, func(t *testing.T) {
			testVaultCompatibility(t, bin, ver)
		})
	}
}

// testVaultCompatibility tests compatibility with the given vault binary
func testVaultCompatibility(t *testing.T, vault string, version string) {
	require := require.New(t)

	// Create a Vault server
	v := testutil.NewTestVaultFromPath(t, vault)
	defer v.Stop()

	token := setupVault(t, v.Client, version)

	// Create a Nomad agent using the created vault
	nomad := agent.NewTestAgent(t, t.Name(), func(c *agent.Config) {
		if c.Vault == nil {
			c.Vault = &config.VaultConfig{}
		}
		c.Vault.Enabled = pointer.Of(true)
		c.Vault.Token = token
		c.Vault.Role = "nomad-cluster"
		c.Vault.AllowUnauthenticated = pointer.Of(true)
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
			require.Fail("too many allocations; something failed")
		}
		alloc := allocs[0]
		//allocID = alloc.ID
		if alloc.ClientStatus == "complete" {
			return true, nil
		}

		return false, fmt.Errorf("client status %q", alloc.ClientStatus)
	}, func(err error) {
		require.NoError(err, "allocation did not finish")
	})

}

// setupVault takes the Vault client and creates the required policies and
// roles. It returns the token that should be used by Nomad
func setupVault(t *testing.T, client *vapi.Client, vaultVersion string) string {
	// Write the policy
	sys := client.Sys()

	// pre-0.9.0 vault servers do not work with our new vault client for the policy endpoint
	// perform this using a raw HTTP request
	newApi := version.Must(version.NewVersion("0.9.0"))
	testVersion := version.Must(version.NewVersion(vaultVersion))
	if testVersion.LessThan(newApi) {
		body := map[string]string{
			"rules": policy,
		}
		request := client.NewRequest("PUT", "/v1/sys/policy/nomad-server")
		if err := request.SetJSONBody(body); err != nil {
			require.NoError(t, err, "failed to set JSON body on legacy policy creation")
		}
		if _, err := client.RawRequest(request); err != nil {
			require.NoError(t, err, "failed to create legacy policy")
		}
	} else {
		if err := sys.PutPolicy("nomad-server", policy); err != nil {
			require.NoError(t, err, "failed to create policy")
		}
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
		require.NoError(t, err, "failed to create child token")
	}

	// Get the client token
	if s == nil || s.Auth == nil {
		require.NoError(t, err, "bad secret response")
	}

	return s.Auth.ClientToken
}
