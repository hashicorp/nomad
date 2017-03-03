package allocdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// TestLinuxSpecialDirs ensures mounting /dev and /proc works.
func TestLinuxSpecialDirs(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Skip("Must be run as root")
	}

	allocDir, err := ioutil.TempDir("", "nomadtest-specialdirs")
	if err != nil {
		t.Fatalf("unable to create tempdir for test: %v", err)
	}
	defer os.RemoveAll(allocDir)

	td := newTaskDir(testLogger(), allocDir, "test")

	// Despite the task dir not existing, unmountSpecialDirs should *not*
	// return an error
	if err := td.unmountSpecialDirs(); err != nil {
		t.Fatalf("error removing nonexistent special dirs: %v", err)
	}

	// Mounting special dirs in a nonexistent task dir *should* return an
	// error
	if err := td.mountSpecialDirs(); err == nil {
		t.Fatalf("expected mounting in a nonexistent task dir %q to fail", td.Dir)
	}

	// Create the task dir like TaskDir.Build would
	if err := os.MkdirAll(td.Dir, 0777); err != nil {
		t.Fatalf("error creating task dir %q: %v", td.Dir, err)
	}

	// Mounting special dirs should now work and contain files
	if err := td.mountSpecialDirs(); err != nil {
		t.Fatalf("error mounting special dirs in %q: %v", td.Dir, err)
	}
	if empty, err := pathEmpty(filepath.Join(td.Dir, "dev")); empty || err != nil {
		t.Fatalf("expected dev to be populated but found: empty=%v error=%v", empty, err)
	}
	if empty, err := pathEmpty(filepath.Join(td.Dir, "proc")); empty || err != nil {
		t.Fatalf("expected proc to be populated but found: empty=%v error=%v", empty, err)
	}

	// Remounting again should be fine
	if err := td.mountSpecialDirs(); err != nil {
		t.Fatalf("error remounting special dirs in %q: %v", td.Dir, err)
	}

	// Now unmount
	if err := td.unmountSpecialDirs(); err != nil {
		t.Fatalf("error unmounting special dirs in %q: %v", td.Dir, err)
	}
	if pathExists(filepath.Join(td.Dir, "dev")) {
		t.Fatalf("dev was not removed from %q", td.Dir)
	}
	if pathExists(filepath.Join(td.Dir, "proc")) {
		t.Fatalf("proc was not removed from %q", td.Dir)
	}
	if err := td.unmountSpecialDirs(); err != nil {
		t.Fatalf("error re-unmounting special dirs in %q: %v", td.Dir, err)
	}
}
