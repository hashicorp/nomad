// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/go-set"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "FAIL: %s\n", err)
		os.Exit(1)
	}
}

// Manifest represents groupings of packages for testing
// see: ci/test-core.json
type Manifest map[string][]string

func (m Manifest) covers(pkg string) bool {
	for _, list := range m {
		if isCovered(list, pkg) {
			return true
		}
	}
	return false
}

const (
	verify = 1
	group  = 2
)

func run(args []string) error {
	mode := len(args)
	if !(mode == verify || mode == group) {
		return errors.New("usage: [filename] <group>")
	}

	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	manifest, err := getManifest(f)
	if err != nil {
		return err
	}

	switch mode {
	case verify:
		return runVerify(manifest)
	case group:
		return runGroups(manifest, args[1])
	default:
		panic("oops")
	}
}

func runVerify(manifest Manifest) error {
	packages, err := inCode(".")
	if err != nil {
		return err
	}

	var isMissing []string
	for _, pkg := range packages {
		if !manifest.covers(pkg) {
			isMissing = append(isMissing, pkg)
		}
	}

	sort.Strings(isMissing)
	for _, pkg := range isMissing {
		_, _ = fmt.Fprintf(os.Stderr, "missing: %s\n", pkg)
	}

	if len(isMissing) > 0 {
		return fmt.Errorf("detected %d packages not tested", len(isMissing))
	}

	return nil
}

func runGroups(manifest Manifest, group string) error {
	list := manifest[group]
	for i := 0; i < len(list); i++ {
		list[i] = "./" + list[i]
	}
	s := strings.Join(list, " ")
	fmt.Print(s)
	return nil
}

// isCovered returns true if pkg is tested by directory in manifest.
func isCovered(coverage []string, pkg string) bool {
	for _, p := range coverage {
		if isCoveredOne(p, pkg) {
			return true
		}
	}
	return false
}

// isCoveredOne returns true if p covers pkg.
//
// p may be a complete path, or a prefix ending with recursive '...'
func isCoveredOne(p string, pkg string) bool {
	if p == pkg {
		return true
	}

	if strings.HasSuffix(p, "/...") {
		prefix := strings.TrimSuffix(p, "/...")
		if strings.HasPrefix(pkg, prefix) {
			return true
		}
	}
	return false
}

func getManifest(r io.Reader) (Manifest, error) {
	m := make(Manifest)
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// uninteresting lists remaining packages that contain Go code but still
// do not need to be covered by test cases.
var uninteresting = []string{
	// module
	"api",

	// main
	".",

	// go embed assets
	"command/asset",

	// testing helpers
	"ci",
	"client/testutil",
	"client/vaultclient",
	"e2e",
	"nomad/mock",
	"plugins/csi/fake",

	// not core code
	"demo",
	"tools",
	"version",
}

func skip(p string) bool {
	for _, prefix := range uninteresting {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func inCode(root string) ([]string, error) {
	pkgs := set.NewTreeSet[string, set.Compare[string]](set.Cmp[string])

	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if skip(path) {
			return nil
		}

		if ext := filepath.Ext(path); ext == ".go" {
			pkgs.Insert(filepath.Dir(path))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	pkgs.Remove(".") // main

	return pkgs.Slice(), nil
}
