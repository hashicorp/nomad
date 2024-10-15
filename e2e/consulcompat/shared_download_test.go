// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consulcompat

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/go-version"
	"github.com/shoenig/test/must"
)

const (
	binDir           = "consul-bins"
	minConsulVersion = "1.16.0"

	// environment variable to pick only one Consul version for testing
	exactConsulVersionEnv = "NOMAD_E2E_CONSULCOMPAT_CONSUL_VERSION"
)

func downloadConsulBuild(t *testing.T, b build, baseDir string) {
	path := filepath.Join(baseDir, binDir, b.Version)
	must.NoError(t, os.MkdirAll(path, 0755))

	if _, err := os.Stat(filepath.Join(path, "consul")); !os.IsNotExist(err) {
		t.Log("download: already have consul at", path)
		return
	}

	t.Log("download: installing consul at", path)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "hc-install", "install",
		"-version", b.Version, "-path", path, "consul")
	bs, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("download: failed to download %s, retrying once: %v", b.Version, err)
		cmd = exec.CommandContext(ctx, "hc-install", "install",
			"-version", b.Version, "-path", path, "consul")
		bs, err = cmd.CombinedOutput()
	}
	must.NoError(t, err, must.Sprintf("failed to download consul %s: %s", b.Version, string(bs)))
}

func getMinimumVersion(t *testing.T) *version.Version {
	v, err := version.NewVersion(minConsulVersion)
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

type consulJSON struct {
	Versions map[string]struct {
		Builds []build `json:"builds"`
	} `json:"versions"`
}

func keep(b build) bool {
	exactVersion := os.Getenv(exactConsulVersionEnv)
	if exactVersion != "" {
		if b.Version != exactVersion {
			return false
		}
	}

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

func scanConsulVersions(t *testing.T, minimum *version.Version) *set.Set[build] {
	httpClient := cleanhttp.DefaultClient()
	httpClient.Timeout = 1 * time.Minute
	response, err := httpClient.Get("https://releases.hashicorp.com/consul/index.json")
	must.NoError(t, err, must.Sprint("unable to download consul versions index"))
	var payload consulJSON
	must.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
	must.Close(t, response.Body)

	// sort the versions for the Y in each consul version X.Y.Z
	// this only works for consul 1.Y.Z which is fine for now
	track := make(tracker)

	for s, obj := range payload.Versions {
		v, err := version.NewVersion(s)
		must.NoError(t, err, must.Sprint("unable to parse consul version"))
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
