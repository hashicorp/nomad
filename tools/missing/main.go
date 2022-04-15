package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gophers.dev/pkgs/ignore"
	"gopkg.in/yaml.v2"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "FAIL: %s\n", err)
		os.Exit(1)
	}
}

type YamlFile struct {
	Jobs struct {
		TestPackages struct {
			Strategy struct {
				Matrix struct {
					Packages []string `yaml:"pkg"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"tests-pkgs"`
	} `yaml:"jobs"`
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("requires filename")
	}

	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer ignore.Close(f)

	coverage, err := inMatrix(f)
	if err != nil {
		return err
	}

	packages, err := inCode(".")
	if err != nil {
		return err
	}

	var isMissing []string
	for _, pkg := range packages {
		if !isCovered(coverage, pkg) {
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

// isCovered returns true if pkg is covered by a package in coverage.
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

func inMatrix(r io.Reader) ([]string, error) {
	var yFile YamlFile
	if err := yaml.NewDecoder(r).Decode(&yFile); err != nil {
		return nil, err
	}
	p := yFile.Jobs.TestPackages.Strategy.Matrix.Packages
	return p, nil
}

type nothing struct{}

var null = nothing{}

// uninteresting lists remaining packages that contain Go code but still
// do not need to be covered by test cases.
var uninteresting = []string{
	// module
	"api",

	// main
	".",

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
	m := map[string]nothing{}

	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if skip(path) {
			return nil
		}

		if ext := filepath.Ext(path); ext == ".go" {
			m[filepath.Dir(path)] = null
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	delete(m, ".") // package main

	var packages []string
	for p := range m {
		packages = append(packages, p)
	}
	sort.Strings(packages)
	return packages, nil
}
