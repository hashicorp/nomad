package secretdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestSecretDir_CreateDestroy(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to make tmpdir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	path := filepath.Join(tmpdir, "secret")
	sdir, err := NewSecretDir(path)
	if err != nil {
		t.Fatalf("Failed to create SecretDir: %v", err)
	}

	// Check the folder exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stating path failed: %v", err)
	}

	if err := sdir.Destroy(); err != nil {
		t.Fatalf("Destroying failed: %v", err)
	}

	// Check the folder doesn't exists
	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		t.Fatalf("path err: %v", err)
	}
}

func TestSecretDir_CreateFor_Remove(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to make tmpdir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	path := filepath.Join(tmpdir, "secret")
	sdir, err := NewSecretDir(path)
	if err != nil {
		t.Fatalf("Failed to create SecretDir: %v", err)
	}

	alloc, task := "123", "foo"
	taskDir, err := sdir.CreateFor(alloc, task)
	if err != nil {
		t.Fatalf("CreateFor failed: %v", err)
	}

	// Check the folder exists
	if _, err := os.Stat(taskDir); err != nil {
		t.Fatalf("Stating path failed: %v", err)
	}

	if err := sdir.Remove(alloc, task); err != nil {
		t.Fatalf("Destroying failed: %v", err)
	}

	// Check the folder doesn't exists
	if _, err := os.Stat(taskDir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("path err: %v", err)
	}
}
