package driver

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLogRotator_IncorrectPath(t *testing.T) {
	incorrectPath := "/foo"

	if _, err := NewLogRotator(incorrectPath, "redis.stdout", 10, 10); err == nil {
		t.Fatal("expected err")
	}
}

func TestLogRotator_FindCorrectIndex(t *testing.T) {
	path := "/tmp/tmplogrator"
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fname := filepath.Join(path, "redis.stdout.1")
	if f, err := os.Create(fname); err == nil {
		f.Close()
	}

	fname = filepath.Join(path, "redis.stdout.2")
	if f, err := os.Create(fname); err == nil {
		f.Close()
	}

	r, err := NewLogRotator(path, "redis.stdout", 10, 10)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	if r.logFileIdx != 2 {
		t.Fatalf("Expected log file idx: %v, actual: %v", 2, r.logFileIdx)
	}
}

func TestLogRotator_AppendToCurrentFile(t *testing.T) {
	path := "/tmp/tmplogrator"
	defer os.RemoveAll(path)
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	fname := filepath.Join(path, "redis.stdout.0")
	if f, err := os.Create(fname); err == nil {
		f.WriteString("abcde")
		f.Close()
	}

	l, err := NewLogRotator(path, "redis.stdout", 10, 6)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	r, w := io.Pipe()
	go func() {
		w.Write([]byte("fg"))
		w.Close()
	}()
	err = l.Start(r)
	if err != nil && err != io.EOF{
		t.Fatal(err)
	}
	finfo, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}
	if finfo.Size() != 6 {
		t.Fatalf("Expected size of file: %v, actual: %v", 6, finfo.Size())
	}
}
