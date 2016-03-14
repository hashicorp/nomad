package logging

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	logger       = log.New(os.Stdout, "", log.LstdFlags)
	pathPrefix   = "logrotator"
	baseFileName = "redis.stdout"
)

func TestFileRotator_IncorrectPath(t *testing.T) {
	if _, err := NewFileRotator("/foo", baseFileName, 10, 10, logger); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFileRotator_CreateNewFile(t *testing.T) {
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	_, err = NewFileRotator(path, baseFileName, 10, 10, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	if _, err := os.Stat(filepath.Join(path, "redis.stdout.0")); err != nil {
		t.Fatalf("expected file")
	}
}

func TestFileRotator_OpenLastFile(t *testing.T) {
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fname1 := filepath.Join(path, "redis.stdout.0")
	fname2 := filepath.Join(path, "redis.stdout.2")
	if _, err := os.Create(fname1); err != nil {
		t.Fatalf("test setup failure: %v", err)
	}
	if _, err := os.Create(fname2); err != nil {
		t.Fatalf("test setup failure: %v", err)
	}

	fr, err := NewFileRotator(path, baseFileName, 10, 10, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	if fr.currentFile.Name() != fname2 {
		t.Fatalf("expected current file: %v, got: %v", fname2, fr.currentFile.Name())
	}
}

func TestFileRotator_WriteToCurrentFile(t *testing.T) {
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fname1 := filepath.Join(path, "redis.stdout.0")
	if _, err := os.Create(fname1); err != nil {
		t.Fatalf("test setup failure: %v", err)
	}

	fr, err := NewFileRotator(path, baseFileName, 10, 5, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	fr.Write([]byte("abcde"))
	time.Sleep(200 * time.Millisecond)
	fi, err := os.Stat(fname1)
	if err != nil {
		t.Fatalf("error getting the file info: %v", err)
	}
	if fi.Size() != 5 {
		t.Fatalf("expected size: %v, actual: %v", 5, fi.Size())
	}
}

func TestFileRotator_RotateFiles(t *testing.T) {
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fr, err := NewFileRotator(path, baseFileName, 10, 5, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	str := "abcdefgh"
	nw, err := fr.Write([]byte(str))
	time.Sleep(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("got error while writing: %v", err)
	}
	if nw != len(str) {
		t.Fatalf("expected %v, got %v", len(str), nw)
	}
	fname1 := filepath.Join(path, "redis.stdout.0")
	fi, err := os.Stat(fname1)
	if err != nil {
		t.Fatalf("error getting the file info: %v", err)
	}
	if fi.Size() != 5 {
		t.Fatalf("expected size: %v, actual: %v", 5, fi.Size())
	}

	fname2 := filepath.Join(path, "redis.stdout.1")
	if _, err := os.Stat(fname2); err != nil {
		t.Fatalf("expected file %v to exist", fname2)
	}

	if fi2, err := os.Stat(fname2); err == nil {
		if fi2.Size() != 3 {
			t.Fatalf("expected size: %v, actual: %v", 3, fi2.Size())
		}
	} else {
		t.Fatalf("error getting the file info: %v", err)
	}
}

func TestFileRotator_WriteRemaining(t *testing.T) {
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fname1 := filepath.Join(path, "redis.stdout.0")
	if f, err := os.Create(fname1); err == nil {
		f.Write([]byte("abcd"))
	} else {
		t.Fatalf("test setup failure: %v", err)
	}

	fr, err := NewFileRotator(path, baseFileName, 10, 5, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	str := "efghijkl"
	nw, err := fr.Write([]byte(str))
	if err != nil {
		t.Fatalf("got error while writing: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if nw != len(str) {
		t.Fatalf("expected %v, got %v", len(str), nw)
	}
	fi, err := os.Stat(fname1)
	if err != nil {
		t.Fatalf("error getting the file info: %v", err)
	}
	if fi.Size() != 5 {
		t.Fatalf("expected size: %v, actual: %v", 5, fi.Size())
	}

	fname2 := filepath.Join(path, "redis.stdout.1")
	if _, err := os.Stat(fname2); err != nil {
		t.Fatalf("expected file %v to exist", fname2)
	}

	if fi2, err := os.Stat(fname2); err == nil {
		if fi2.Size() != 5 {
			t.Fatalf("expected size: %v, actual: %v", 5, fi2.Size())
		}
	} else {
		t.Fatalf("error getting the file info: %v", err)
	}

	fname3 := filepath.Join(path, "redis.stdout.2")
	if _, err := os.Stat(fname3); err != nil {
		t.Fatalf("expected file %v to exist", fname3)
	}

	if fi3, err := os.Stat(fname3); err == nil {
		if fi3.Size() != 2 {
			t.Fatalf("expected size: %v, actual: %v", 2, fi3.Size())
		}
	} else {
		t.Fatalf("error getting the file info: %v", err)
	}

}

func TestFileRotator_PurgeOldFiles(t *testing.T) {
	var path string
	var err error
	if path, err = ioutil.TempDir("", pathPrefix); err != nil {
		t.Fatalf("test setup err: %v", err)
	}
	defer os.RemoveAll(path)

	fr, err := NewFileRotator(path, baseFileName, 2, 2, logger)
	if err != nil {
		t.Fatalf("test setup err: %v", err)
	}

	str := "abcdeghijklmn"
	nw, err := fr.Write([]byte(str))
	if err != nil {
		t.Fatalf("got error while writing: %v", err)
	}
	if nw != len(str) {
		t.Fatalf("expected %v, got %v", len(str), nw)
	}

	time.Sleep(1 * time.Second)
	f, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatalf("test error: %v", err)
	}

	if len(f) != 2 {
		t.Fatalf("expected number of files: %v, got: %v", 2, len(f))
	}
}
