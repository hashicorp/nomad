package driver

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	logger = log.New(os.Stdout, "", log.LstdFlags)
	path   = "/tmp/logrotator"
)

func TestLogRotator_InvalidPath(t *testing.T) {
	invalidPath := "/foo"

	if _, err := NewLogRotator(invalidPath, "redis.stdout", 10, 10, logger); err == nil {
		t.Fatal("expected err")
	}
}

func TestLogRotator_FindCorrectIndex(t *testing.T) {
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

	r, err := NewLogRotator(path, "redis.stdout", 10, 10, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	if r.logFileIdx != 2 {
		t.Fatalf("Expected log file idx: %v, actual: %v", 2, r.logFileIdx)
	}
}

func TestLogRotator_AppendToCurrentFile(t *testing.T) {
	defer os.RemoveAll(path)
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	fname := filepath.Join(path, "redis.stdout.0")
	if f, err := os.Create(fname); err == nil {
		f.WriteString("abcde")
		f.Close()
	}

	l, err := NewLogRotator(path, "redis.stdout", 10, 6, logger)
	if err != nil && err != io.EOF {
		t.Fatalf("test setup err: %v", err)
	}

	r, w := io.Pipe()
	go func() {
		w.Write([]byte("fg"))
		w.Close()
	}()
	err = l.Start(r)
	if err != nil && err != io.EOF {
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

func TestLogRotator_RotateFiles(t *testing.T) {
	defer os.RemoveAll(path)
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	fname := filepath.Join(path, "redis.stdout.0")
	if f, err := os.Create(fname); err == nil {
		f.WriteString("abcde")
		f.Close()
	}

	l, err := NewLogRotator(path, "redis.stdout", 10, 6, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	r, w := io.Pipe()
	go func() {
		w.Write([]byte("fg"))
		w.Close()
	}()
	err = l.Start(r)
	if err != nil && err != io.EOF {
		t.Fatalf("Failure in logrotator start %v", err)
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	numFiles := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "redis.stdout.") {
			numFiles += 1
		}
	}

	if numFiles != 2 {
		t.Fatalf("Expected number of files: %v, actual: %v", 2, numFiles)
	}
}

func TestLogRotator_StartFromEmptyDir(t *testing.T) {
	defer os.RemoveAll(path)
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	l, err := NewLogRotator(path, "redis.stdout", 10, 10, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	r, w := io.Pipe()
	go func() {
		w.Write([]byte("abcdefg"))
		w.Close()
	}()
	err = l.Start(r)
	if err != nil && err != io.EOF {
		t.Fatalf("Failure in logrotator start %v", err)
	}

	finfo, err := os.Stat(filepath.Join(path, "redis.stdout.0"))
	if err != nil {
		t.Fatal(err)
	}
	if finfo.Size() != 7 {
		t.Fatalf("expected size of file: %v, actual: %v", 7, finfo.Size())
	}

}

func TestLogRotator_SetPathAsFile(t *testing.T) {
	defer os.RemoveAll(path)
	if _, err := os.Create(path); err != nil {
		t.Fatalf("test setup problem: %v", err)
	}

	_, err := NewLogRotator(path, "redis.stdout", 10, 10, logger)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLogRotator_ExcludeDirs(t *testing.T) {
	defer os.RemoveAll(path)
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	if err := os.Mkdir(filepath.Join(path, "redis.stdout.0"), os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	l, err := NewLogRotator(path, "redis.stdout", 10, 6, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	r, w := io.Pipe()
	go func() {
		w.Write([]byte("fg"))
		w.Close()
	}()
	err = l.Start(r)
	if err != nil && err != io.EOF {
		t.Fatalf("Failure in logrotator start %v", err)
	}

	finfo, err := os.Stat(filepath.Join(path, "redis.stdout.1"))
	if err != nil {
		t.Fatal("expected rotator to create redis.stdout.1")
	}
	if finfo.Size() != 2 {
		t.Fatalf("expected size: %v, actual: %v", 2, finfo.Size())
	}
}

func TestLogRotator_PurgeDirs(t *testing.T) {
	defer os.RemoveAll(path)
	if err := os.Mkdir(path, os.ModeDir|os.ModePerm); err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	l, err := NewLogRotator(path, "redis.stdout", 2, 4, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	r, w := io.Pipe()
	go func() {
		w.Write([]byte("abcdefghijklmno"))
		w.Close()
	}()

	err = l.Start(r)
	if err != nil && err != io.EOF {
		t.Fatalf("failure in logrotator start: %v", err)
	}
	l.PurgeOldFiles()

	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected number of files: %v, actual: %v", 2, len(files))
	}
}
