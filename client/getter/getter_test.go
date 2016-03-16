package getter

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestGetArtifact_FileAndChecksum(t *testing.T) {
	// Create the test server hosting the file to download
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("./test-fixtures/"))))
	defer ts.Close()

	// Create a temp directory to download into
	destDir, err := ioutil.TempDir("", "nomad-test")
	if err != nil {
		t.Fatalf("failed to make temp directory: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create the artifact
	file := "test.sh"
	artifact := &structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, file),
		GetterOptions: map[string]string{
			"checksum": "md5:bce963762aa2dbfed13caf492a45fb72",
		},
	}

	// Download the artifact
	logger := log.New(os.Stderr, "", log.LstdFlags)
	if err := GetArtifact(artifact, destDir, logger); err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}

	// Verify artifact exists
	if _, err := os.Stat(filepath.Join(destDir, file)); err != nil {
		t.Fatalf("source path error: %s", err)
	}
}

func TestGetArtifact_InvalidChecksum(t *testing.T) {
	// Create the test server hosting the file to download
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("./test-fixtures/"))))
	defer ts.Close()

	// Create a temp directory to download into
	destDir, err := ioutil.TempDir("", "nomad-test")
	if err != nil {
		t.Fatalf("failed to make temp directory: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create the artifact with an incorrect checksum
	file := "test.sh"
	artifact := &structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, file),
		GetterOptions: map[string]string{
			"checksum": "md5:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	// Download the artifact and expect an error
	logger := log.New(os.Stderr, "", log.LstdFlags)
	if err := GetArtifact(artifact, destDir, logger); err == nil {
		t.Fatalf("GetArtifact should have failed")
	}
}

func createContents(basedir string, fileContents map[string]string, t *testing.T) {
	for relPath, content := range fileContents {
		folder := basedir
		if strings.Index(relPath, "/") != -1 {
			// Create the folder.
			folder = filepath.Join(basedir, filepath.Dir(relPath))
			if err := os.Mkdir(folder, 0777); err != nil {
				t.Fatalf("failed to make directory: %v", err)
			}
		}

		// Create a file in the existing folder.
		file := filepath.Join(folder, filepath.Base(relPath))
		if err := ioutil.WriteFile(file, []byte(content), 0777); err != nil {
			t.Fatalf("failed to write data to file %v: %v", file, err)
		}
	}
}

func checkContents(basedir string, fileContents map[string]string, t *testing.T) {
	for relPath, content := range fileContents {
		path := filepath.Join(basedir, relPath)
		actual, err := ioutil.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file %q: %v", path, err)
		}

		if !reflect.DeepEqual(actual, []byte(content)) {
			t.Fatalf("%q: expected %q; got %q", path, content, string(actual))
		}
	}
}

func TestGetArtifact_Archive(t *testing.T) {
	// Create the test server hosting the file to download
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("./test-fixtures/"))))
	defer ts.Close()

	// Create a temp directory to download into and create some of the same
	// files that exist in the artifact to ensure they are overriden
	destDir, err := ioutil.TempDir("", "nomad-test")
	if err != nil {
		t.Fatalf("failed to make temp directory: %v", err)
	}
	defer os.RemoveAll(destDir)

	create := map[string]string{
		"exist/my.config": "to be replaced",
		"untouched":       "existing top-level",
	}
	createContents(destDir, create, t)

	file := "archive.tar.gz"
	artifact := &structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, file),
		GetterOptions: map[string]string{
			"checksum": "sha1:20bab73c72c56490856f913cf594bad9a4d730f6",
		},
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	if err := GetArtifact(artifact, destDir, logger); err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}

	// Verify the unarchiving overrode files properly.
	expected := map[string]string{
		"untouched":       "existing top-level",
		"exist/my.config": "hello world\n",
		"new/my.config":   "hello world\n",
		"test.sh":         "sleep 1\n",
	}
	checkContents(destDir, expected, t)
}
