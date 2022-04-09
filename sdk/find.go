package sdk

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Find will attempt to locate executable by searching in a few common places in
// a sensible order for the context of a test case.
//
// Find will search for executable in the order of
// - the current process
// - in $PATH
// - in $GOPATH/bin
// - in $CWD
// - in $CWD/bin
// - in $GOBIN
func Find(t *testing.T, executable string) string {
	target := forPlatform(executable)

	// Check the current executable
	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to determine this process: %v", err)
	}

	// Not including test executables
	if _, err = os.Stat(bin); err == nil && isTarget(bin, target) {
		return bin
	}

	// Look on $PATH
	if bin, err = exec.LookPath(target); err == nil {
		return bin
	}

	// Look on $GOPATH
	bin = filepath.Join(os.Getenv("GOPATH"), "bin", target)
	if _, err = os.Stat(bin); err == nil {
		return bin
	}

	// Look in CWD
	pwd, _ := os.Getwd()
	bin = filepath.Join(pwd, target)
	if _, err = os.Stat(bin); err == nil {
		return bin
	}

	// Look in CWD/bin
	bin = filepath.Join(pwd, "bin", target)
	if _, err = os.Stat(bin); err == nil {
		return bin
	}

	// Look on $GOBIN
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		bin = filepath.Join(gobin, target)
		if _, err = os.Stat(bin); err == nil {
			return bin
		}
	}

	t.Fatalf("failed to find executable %q", target)
	return ""
}

func isTarget(path, target string) bool {
	if strings.HasSuffix(path, ".test") || strings.HasSuffix(path, ".test.exe") {
		return false
	}
	return true
}

func forPlatform(name string) string {
	if runtime.GOOS == "windows" && !(filepath.Ext(name) == ".exe") {
		return name + ".exec"
	}
	return name
}
