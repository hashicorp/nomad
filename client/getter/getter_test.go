package getter

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// fakeReplacer is a noop version of env.TaskEnv.ReplanceEnv
type fakeReplacer struct{}

func (fakeReplacer) ReplaceEnv(s string) string {
	return s
}

var taskEnv = fakeReplacer{}

func TestGetArtifact_FileAndChecksum(t *testing.T) {
	// Create the test server hosting the file to download
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("./test-fixtures/"))))
	defer ts.Close()

	// Create a temp directory to download into
	taskDir, err := ioutil.TempDir("", "nomad-test")
	if err != nil {
		t.Fatalf("failed to make temp directory: %v", err)
	}
	defer os.RemoveAll(taskDir)

	// Create the artifact
	file := "test.sh"
	artifact := &structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, file),
		GetterOptions: map[string]string{
			"checksum": "md5:bce963762aa2dbfed13caf492a45fb72",
		},
	}

	// Download the artifact
	if err := GetArtifact(taskEnv, artifact, taskDir); err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}

	// Verify artifact exists
	if _, err := os.Stat(filepath.Join(taskDir, file)); err != nil {
		t.Fatalf("file not found: %s", err)
	}
}

func TestGetArtifact_File_RelativeDest(t *testing.T) {
	// Create the test server hosting the file to download
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("./test-fixtures/"))))
	defer ts.Close()

	// Create a temp directory to download into
	taskDir, err := ioutil.TempDir("", "nomad-test")
	if err != nil {
		t.Fatalf("failed to make temp directory: %v", err)
	}
	defer os.RemoveAll(taskDir)

	// Create the artifact
	file := "test.sh"
	relative := "foo/"
	artifact := &structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, file),
		GetterOptions: map[string]string{
			"checksum": "md5:bce963762aa2dbfed13caf492a45fb72",
		},
		RelativeDest: relative,
	}

	// Download the artifact
	if err := GetArtifact(taskEnv, artifact, taskDir); err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}

	// Verify artifact was downloaded to the correct path
	if _, err := os.Stat(filepath.Join(taskDir, relative, file)); err != nil {
		t.Fatalf("file not found: %s", err)
	}
}

func TestGetGetterUrl_Interprolation(t *testing.T) {
	// Create the artifact
	artifact := &structs.TaskArtifact{
		GetterSource: "${NOMAD_META_ARTIFACT}",
	}

	url := "foo.com"
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Meta = map[string]string{"artifact": url}
	taskEnv := env.NewBuilder(mock.Node(), alloc, task, "global").Build()

	act, err := getGetterUrl(taskEnv, artifact)
	if err != nil {
		t.Fatalf("getGetterUrl() failed: %v", err)
	}

	if act != url {
		t.Fatalf("getGetterUrl() returned %q; want %q", act, url)
	}
}

func TestGetArtifact_InvalidChecksum(t *testing.T) {
	// Create the test server hosting the file to download
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("./test-fixtures/"))))
	defer ts.Close()

	// Create a temp directory to download into
	taskDir, err := ioutil.TempDir("", "nomad-test")
	if err != nil {
		t.Fatalf("failed to make temp directory: %v", err)
	}
	defer os.RemoveAll(taskDir)

	// Create the artifact with an incorrect checksum
	file := "test.sh"
	artifact := &structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, file),
		GetterOptions: map[string]string{
			"checksum": "md5:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	// Download the artifact and expect an error
	if err := GetArtifact(taskEnv, artifact, taskDir); err == nil {
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
	// files that exist in the artifact to ensure they are overridden
	taskDir, err := ioutil.TempDir("", "nomad-test")
	if err != nil {
		t.Fatalf("failed to make temp directory: %v", err)
	}
	defer os.RemoveAll(taskDir)

	create := map[string]string{
		"exist/my.config": "to be replaced",
		"untouched":       "existing top-level",
	}
	createContents(taskDir, create, t)

	file := "archive.tar.gz"
	artifact := &structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, file),
		GetterOptions: map[string]string{
			"checksum": "sha1:20bab73c72c56490856f913cf594bad9a4d730f6",
		},
	}

	if err := GetArtifact(taskEnv, artifact, taskDir); err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}

	// Verify the unarchiving overrode files properly.
	expected := map[string]string{
		"untouched":       "existing top-level",
		"exist/my.config": "hello world\n",
		"new/my.config":   "hello world\n",
		"test.sh":         "sleep 1\n",
	}
	checkContents(taskDir, expected, t)
}

func TestGetGetterUrl_Queries(t *testing.T) {
	cases := []struct {
		name     string
		artifact *structs.TaskArtifact
		output   string
	}{
		{
			name: "adds query parameters",
			artifact: &structs.TaskArtifact{
				GetterSource: "https://foo.com?test=1",
				GetterOptions: map[string]string{
					"foo": "bar",
					"bam": "boom",
				},
			},
			output: "https://foo.com?bam=boom&foo=bar&test=1",
		},
		{
			name: "git without http",
			artifact: &structs.TaskArtifact{
				GetterSource: "github.com/hashicorp/nomad",
				GetterOptions: map[string]string{
					"ref": "abcd1234",
				},
			},
			output: "github.com/hashicorp/nomad?ref=abcd1234",
		},
		{
			name: "git using ssh",
			artifact: &structs.TaskArtifact{
				GetterSource: "git@github.com:hashicorp/nomad?sshkey=1",
				GetterOptions: map[string]string{
					"ref": "abcd1234",
				},
			},
			output: "git@github.com:hashicorp/nomad?ref=abcd1234&sshkey=1",
		},
		{
			name: "s3 scheme 1",
			artifact: &structs.TaskArtifact{
				GetterSource: "s3::https://s3.amazonaws.com/bucket/foo",
				GetterOptions: map[string]string{
					"aws_access_key_id": "abcd1234",
				},
			},
			output: "s3::https://s3.amazonaws.com/bucket/foo?aws_access_key_id=abcd1234",
		},
		{
			name: "s3 scheme 2",
			artifact: &structs.TaskArtifact{
				GetterSource: "s3::https://s3-eu-west-1.amazonaws.com/bucket/foo",
				GetterOptions: map[string]string{
					"aws_access_key_id": "abcd1234",
				},
			},
			output: "s3::https://s3-eu-west-1.amazonaws.com/bucket/foo?aws_access_key_id=abcd1234",
		},
		{
			name: "s3 scheme 3",
			artifact: &structs.TaskArtifact{
				GetterSource: "bucket.s3.amazonaws.com/foo",
				GetterOptions: map[string]string{
					"aws_access_key_id": "abcd1234",
				},
			},
			output: "bucket.s3.amazonaws.com/foo?aws_access_key_id=abcd1234",
		},
		{
			name: "s3 scheme 4",
			artifact: &structs.TaskArtifact{
				GetterSource: "bucket.s3-eu-west-1.amazonaws.com/foo/bar",
				GetterOptions: map[string]string{
					"aws_access_key_id": "abcd1234",
				},
			},
			output: "bucket.s3-eu-west-1.amazonaws.com/foo/bar?aws_access_key_id=abcd1234",
		},
		{
			name: "local file",
			artifact: &structs.TaskArtifact{
				GetterSource: "/foo/bar",
			},
			output: "/foo/bar",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			act, err := getGetterUrl(taskEnv, c.artifact)
			if err != nil {
				t.Fatalf("want %q; got err %v", c.output, err)
			} else if act != c.output {
				t.Fatalf("want %q; got %q", c.output, act)
			}
		})
	}
}
