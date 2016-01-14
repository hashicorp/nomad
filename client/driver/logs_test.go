package driver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogRotator_IncorrectPath(t *testing.T) {
	incorrectPath := "/foo"

	if _, err := NewLogRotor(incorrectPath, "redis.stdout", 10, 10); err == nil {
		t.Fatal("expected err")
	}
}

func TestLogRotator_FindCorrectIndex(t *testing.T) {
	path := "/tmp/tmplogrator"
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	fname := filepath.Join(path, "redis.stdout.1")
	if f, err := os.Create(fname); err == nil {
		f.Close()
	}

	fname = filepath.Join(path, "redis.stdout.2")
	if f, err := os.Create(fname); err == nil {
		f.Close()
	}

	r, err := NewLogRotor(path, "redis.stdout", 10, 10)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	if r.logFileIdx != 2 {
		t.Fatalf("Expected log file idx: %v, actual: %v", 2, r.logFileIdx)
	}
}
