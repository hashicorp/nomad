package vault

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/go-version"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	vapi "github.com/hashicorp/vault/api"
)

var (
	minVaultVer = version.Must(version.NewVersion("0.6.2"))
)

// syncVault discovers available versions of Vault, downloads the binaries,
// returns a map of version to binary path.
func syncVault(t *testing.T) map[string]string {

	binDir := filepath.Join(os.TempDir(), "vault-bins/")

	versions := vaultVersions(t)

	// Get the binaries we need to download
	missing, err := missingVault(binDir, versions)
	require.NoError(t, err)

	// Create the directory for the binaries
	require.NoError(t, createBinDir(binDir))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Limit to N concurrent downloads
	sema := make(chan int, 5)

	// Download in parallel
	start := time.Now()
	g, _ := errgroup.WithContext(ctx)
	for ver, url := range missing {
		dst := filepath.Join(binDir, ver)
		g.Go(func() error {
			sema <- 1
			defer func() {
				<-sema
			}()
			return getVault(dst, url)
		})
	}
	require.NoError(t, g.Wait())
	if n := len(missing); n > 0 {
		t.Logf("Downloaded %d versions of Vault in %s", n, time.Now().Sub(start))
	}

	binaries := make(map[string]string, len(versions))
	for ver, _ := range versions {
		binaries[ver] = filepath.Join(binDir, ver)
	}
	return binaries
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
	files, err := ioutil.ReadDir(binDir)
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
	if os.Getenv("NOMAD_E2E") == "" {
		t.Skip("Skipping e2e tests, NOMAD_E2E not set")
	}

	vaultBinaries := syncVault(t)

	for version, vaultBin := range vaultBinaries {
		ver := version
		bin := vaultBin
		t.Run(version, func(t *testing.T) {
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
func setupVault(t *testing.T, client *vapi.Client, vaultVersion string) string {
	// Write the policy
	sys := client.Sys()

	// pre-0.9.0 vault servers do not work with our new vault client for the policy endpoint
	// perform this using a raw HTTP request
	newApi, _ := version.NewVersion("0.9.0")
	testVersion, err := version.NewVersion(vaultVersion)
	if err != nil {
		t.Fatalf("failed to parse test version from '%v': %v", t.Name(), err)
	}
	if testVersion.LessThan(newApi) {
		body := map[string]string{
			"rules": policy,
		}
		request := client.NewRequest("PUT", fmt.Sprintf("/v1/sys/policy/%s", "nomad-server"))
		if err := request.SetJSONBody(body); err != nil {
			t.Fatalf("failed to set JSON body on legacy policy creation: %v", err)
		}
		if _, err := client.RawRequest(request); err != nil {
			t.Fatalf("failed to create legacy policy: %v", err)
		}
	} else {
		if err := sys.PutPolicy("nomad-server", policy); err != nil {
			t.Fatalf("failed to create policy: %v", err)
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
		t.Fatalf("failed to create child token: %v", err)
	}

	// Get the client token
	if s == nil || s.Auth == nil {
		t.Fatalf("bad secret response: %+v", s)
	}

	return s.Auth.ClientToken
}
