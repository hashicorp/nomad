package allocdir

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

var notFoundErr = fmt.Errorf("not found")

func isMount(path string) error {
	file, err := os.Open("/proc/self/mounts")
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64*1024)
	const max = 100000
	for i := 0; i < max; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return notFoundErr
			}
			return err
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			return fmt.Errorf("unexpected line: %q", line)
		}
		if parts[1] == path {
			// Found it! Make sure it's a tmpfs
			if parts[0] != "tmpfs" {
				return fmt.Errorf("unexpected fs: %q", parts[1])
			}
			return nil
		}
	}
	return fmt.Errorf("exceeded max mount entries (%d)", max)
}

// TestLinuxRootSecretDir asserts secret dir creation and removal are
// idempotent.
func TestLinuxRootSecretDir(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Skip("Must be run as root")
	}
	tmpdir, err := ioutil.TempDir("", "nomadtest-rootsecretdir")
	if err != nil {
		t.Fatalf("unable to create tempdir for test: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	secretsDir := filepath.Join(tmpdir, TaskSecrets)

	// removing a nonexistent secrets dir should NOT error
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
	// run twice as it should be idempotent
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}

	// creating a secrets dir should work
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}
	// creating it again should be a noop (NO error)
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}

	// ensure it exists and is a directory
	fi, err := os.Lstat(secretsDir)
	if err != nil {
		t.Fatalf("error stat'ing secrets dir %q: %v", secretsDir, err)
	}
	if !fi.IsDir() {
		t.Fatalf("secrets dir %q is not a directory and should be", secretsDir)
	}
	if err := isMount(secretsDir); err != nil {
		t.Fatalf("secrets dir %q is not a mount: %v", secretsDir, err)
	}

	// now remove it
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing secrets dir %q: %v", secretsDir, err)
	}

	// make sure it's gone
	if err := isMount(secretsDir); err != notFoundErr {
		t.Fatalf("error ensuring secrets dir %q isn't mounted: %v", secretsDir, err)
	}

	// removing again should be a noop
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
}

// TestLinuxUnprivilegedSecretDir asserts secret dir creation and removal are
// idempotent.
func TestLinuxUnprivilegedSecretDir(t *testing.T) {
	if unix.Geteuid() == 0 {
		t.Skip("Must not be run as root")
	}
	tmpdir, err := ioutil.TempDir("", "nomadtest-secretdir")
	if err != nil {
		t.Fatalf("unable to create tempdir for test: %s", err)
	}
	defer os.RemoveAll(tmpdir)

	secretsDir := filepath.Join(tmpdir, TaskSecrets)

	// removing a nonexistent secrets dir should NOT error
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
	// run twice as it should be idempotent
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}

	// creating a secrets dir should work
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}
	// creating it again should be a noop (NO error)
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}

	// ensure it exists and is a directory
	fi, err := os.Lstat(secretsDir)
	if err != nil {
		t.Fatalf("error stat'ing secrets dir %q: %v", secretsDir, err)
	}
	if !fi.IsDir() {
		t.Fatalf("secrets dir %q is not a directory and should be", secretsDir)
	}
	if err := isMount(secretsDir); err != notFoundErr {
		t.Fatalf("error ensuring secrets dir %q isn't mounted: %v", secretsDir, err)
	}

	// now remove it
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing secrets dir %q: %v", secretsDir, err)
	}

	// make sure it's gone
	if _, err := os.Lstat(secretsDir); err == nil {
		t.Fatalf("expected secrets dir %q to be gone but it was found", secretsDir)
	}

	// removing again should be a noop
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
}

func TestLinuxRoot_BindMounting_DirRW(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Skip("Must be run as root")
	}

	tmpdir, err := ioutil.TempDir("", "test-linux-root")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	sourceDir := filepath.Join(tmpdir, "sourcedir")
	require.NoError(t, os.Mkdir(sourceDir, 0601))

	sampleFileContent := randomBytes(30)
	err = ioutil.WriteFile(
		filepath.Join(sourceDir, "testfile"),
		sampleFileContent,
		0604)
	require.NoError(t, err)

	target := filepath.Join(tmpdir, "targetdir-rw")
	defer unmount(target)

	err = bindMount(sourceDir, target, false)
	require.NoError(t, err)

	// resulting target has the same permissions
	fi, err := os.Stat(target)
	require.NoError(t, err)
	require.EqualValues(t, os.FileMode(0601), fi.Mode().Perm())

	// can see the file in source reference through mount
	found, err := ioutil.ReadFile(filepath.Join(target, "testfile"))
	require.NoError(t, err)
	require.EqualValues(t, sampleFileContent, found)

	fi, err = os.Stat(filepath.Join(target, "testfile"))
	require.NoError(t, err)
	require.EqualValues(t, os.FileMode(0604), fi.Mode().Perm())

	// can modify in target path and work is present through source
	newFileContent := randomBytes(30)
	err = ioutil.WriteFile(filepath.Join(target, "newtestfile"),
		newFileContent,
		0604)
	require.NoError(t, err)

	found, err = ioutil.ReadFile(filepath.Join(sourceDir, "newtestfile"))
	require.NoError(t, err)
	require.EqualValues(t, newFileContent, found)

	// unmount
	err = unmount(target)
	require.NoError(t, err)

	// once unmount, cannot read files through target
	_, err = ioutil.ReadFile(filepath.Join(target, "testfile"))
	require.True(t, os.IsNotExist(err))
}

func TestLinuxRoot_BindMounting_DirRO(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Skip("Must be run as root")
	}

	tmpdir, err := ioutil.TempDir("", "test-linux-root")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	sourceDir := filepath.Join(tmpdir, "sourcedir")
	require.NoError(t, os.Mkdir(sourceDir, 0601))

	sampleFileContent := randomBytes(30)
	err = ioutil.WriteFile(
		filepath.Join(sourceDir, "testfile"),
		sampleFileContent,
		0604)
	require.NoError(t, err)

	target := filepath.Join(tmpdir, "targetdir-ro")
	fmt.Printf("created path %v\n", target)
	defer unmount(target)

	err = bindMount(sourceDir, target, true)
	require.NoError(t, err)

	// resulting target has the same permissions
	fi, err := os.Stat(target)
	require.NoError(t, err)
	require.EqualValues(t, os.FileMode(0601), fi.Mode().Perm())

	// can see the file in source reference through mount
	found, err := ioutil.ReadFile(filepath.Join(target, "testfile"))
	require.NoError(t, err)
	require.EqualValues(t, sampleFileContent, found)

	fi, err = os.Stat(filepath.Join(target, "testfile"))
	require.NoError(t, err)
	require.EqualValues(t, os.FileMode(0604), fi.Mode().Perm())

	// cannot write to target path
	err = ioutil.WriteFile(filepath.Join(target, "newtestfile"),
		randomBytes(30),
		0604)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read-only file system")

	// unmount
	err = unmount(target)
	require.NoError(t, err)

	// once unmount, cannot read files through target
	_, err = ioutil.ReadFile(filepath.Join(target, "testfile"))
	require.True(t, os.IsNotExist(err))
}

func TestLinuxRoot_BindMounting_FileRW(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Skip("Must be run as root")
	}

	tmpdir, err := ioutil.TempDir("", "test-linux-root")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	source := filepath.Join(tmpdir, "sourcefile")

	sampleFileContent := randomBytes(30)
	err = ioutil.WriteFile(source, sampleFileContent, 0604)
	require.NoError(t, err)

	target := filepath.Join(tmpdir, "sourcefile-rw")
	defer unmount(target)

	err = bindMount(source, target, false)
	require.NoError(t, err)

	// resulting target has the same permissions
	fi, err := os.Stat(target)
	require.NoError(t, err)
	require.EqualValues(t, os.FileMode(0604), fi.Mode().Perm())

	// can see the file in source reference through mount
	found, err := ioutil.ReadFile(target)
	require.NoError(t, err)
	require.EqualValues(t, sampleFileContent, found)

	// can modify in target path and work is present through source
	newFileContent := randomBytes(30)
	err = ioutil.WriteFile(target,
		newFileContent,
		0604)
	require.NoError(t, err)

	found, err = ioutil.ReadFile(source)
	require.NoError(t, err)
	require.EqualValues(t, newFileContent, found)

	// unmount
	err = unmount(target)
	require.NoError(t, err)

	// Once unmount, the target should appear empty.
	// The file may not necessarily be deleted
	found, _ = ioutil.ReadFile(target)
	require.Len(t, found, 0)
}

func TestLinuxRoot_BindMounting_FileRO(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Skip("Must be run as root")
	}

	tmpdir, err := ioutil.TempDir("", "test-linux-root")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	source := filepath.Join(tmpdir, "sourcefile")

	sampleFileContent := randomBytes(30)
	err = ioutil.WriteFile(source, sampleFileContent, 0604)
	require.NoError(t, err)

	target := filepath.Join(tmpdir, "sourcefile-ro")
	defer unmount(target)

	err = bindMount(source, target, true)
	require.NoError(t, err)

	// resulting target has the same permissions
	fi, err := os.Stat(target)
	require.NoError(t, err)
	require.EqualValues(t, os.FileMode(0604), fi.Mode().Perm())

	// can see the file in source reference through mount
	found, err := ioutil.ReadFile(target)
	require.NoError(t, err)
	require.EqualValues(t, sampleFileContent, found)

	// can modify in target path and work is present through source
	err = ioutil.WriteFile(target,
		randomBytes(30),
		0604)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read-only file system")

	// unmount
	err = unmount(target)
	require.NoError(t, err)

	// Once unmount, the target should appear empty.
	// The file may not necessarily be deleted
	found, _ = ioutil.ReadFile(target)
	require.Len(t, found, 0)
}

func randomBytes(c int) []byte {
	b := make([]byte, c)
	_, err := rand.Read(b)
	if err != nil {
		panic("could not generate random bytes")
	}

	return b
}
