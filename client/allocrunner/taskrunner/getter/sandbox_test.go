// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"archive/tar"
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

const testFileContent = "test-file-content"

func artifactConfig(timeout time.Duration) *config.ArtifactConfig {
	return &config.ArtifactConfig{
		HTTPReadTimeout: timeout,
		HTTPMaxBytes:    1e6,
		GCSTimeout:      timeout,
		GitTimeout:      timeout,
		HgTimeout:       timeout,
		S3Timeout:       timeout,
	}
}

// comprehensive scenarios tested in e2e/artifact

func TestSandbox_Get_http(t *testing.T) {
	testutil.RequireRoot(t)
	logger := testlog.HCLogger(t)

	ac := artifactConfig(10 * time.Second)
	sbox := New(ac, logger)

	_, taskDir := SetupDir(t)
	env := noopTaskEnv(taskDir)

	artifact := &structs.TaskArtifact{
		GetterSource: "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod",
		RelativeDest: "local/downloads",
	}

	err := sbox.Get(env, artifact, "nobody")
	must.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(taskDir, "local", "downloads", "go.mod"))
	must.NoError(t, err)
	must.StrContains(t, string(b), "module github.com/hashicorp/go-set")
}

func TestSandbox_Get_insecure_http(t *testing.T) {
	testutil.RequireRoot(t)
	logger := testlog.HCLogger(t)

	ac := artifactConfig(10 * time.Second)
	sbox := New(ac, logger)

	_, taskDir := SetupDir(t)
	env := noopTaskEnv(taskDir)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	artifact := &structs.TaskArtifact{
		GetterSource: srv.URL,
		RelativeDest: "local/downloads",
	}

	err := sbox.Get(env, artifact, "nobody")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "x509: certificate signed by unknown authority")

	artifact.GetterInsecure = true
	err = sbox.Get(env, artifact, "nobody")
	must.NoError(t, err)
}

func TestSandbox_Get_chown(t *testing.T) {
	testutil.RequireRoot(t)
	logger := testlog.HCLogger(t)

	ac := artifactConfig(10 * time.Second)
	sbox := New(ac, logger)

	_, taskDir := SetupDir(t)
	env := noopTaskEnv(taskDir)

	artifact := &structs.TaskArtifact{
		GetterSource: "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod",
		RelativeDest: "local/downloads",
		Chown:        true,
	}

	err := sbox.Get(env, artifact, "nobody")
	must.NoError(t, err)

	info, err := os.Stat(filepath.Join(taskDir, "local", "downloads"))
	must.NoError(t, err)

	uid := info.Sys().(*syscall.Stat_t).Uid
	must.Eq(t, 65534, uid) // nobody's conventional uid
}

func TestSandbox_Get_inspection(t *testing.T) {
	// These tests disable filesystem isolation as the
	// artifact inspection is what is being tested.
	testutil.RequireRoot(t)
	logger := testlog.HCLogger(t)

	sandboxSetup := func() (string, *Sandbox, interfaces.EnvReplacer) {
		testutil.RequireRoot(t)
		logger := testlog.HCLogger(t)
		ac := artifactConfig(10 * time.Second)
		sbox := New(ac, logger)
		_, taskDir := SetupDir(t)
		env := noopTaskEnv(taskDir)
		sbox.ac.DisableFilesystemIsolation = true

		return taskDir, sbox, env
	}

	t.Run("symlink escaped sandbox", func(t *testing.T) {
		dir, err := os.MkdirTemp(t.TempDir(), "fake-repo")
		must.NoError(t, err, must.Sprint("failed to create local repo directory"))
		must.NoError(t, os.Symlink("/", filepath.Join(dir, "bad-file")), must.Sprint("could not create symlink in local repo"))
		srv := makeAndServeGitRepo(t, dir)

		artifact := &structs.TaskArtifact{
			RelativeDest: "local/symlink",
			GetterSource: fmt.Sprintf("git::%s/%s", srv.URL, filepath.Base(dir)),
		}

		t.Run("default", func(t *testing.T) {
			ac := artifactConfig(10 * time.Second)
			sbox := New(ac, logger)

			_, taskDir := SetupDir(t)
			env := noopTaskEnv(taskDir)
			sbox.ac.DisableFilesystemIsolation = true

			err := sbox.Get(env, artifact, "nobody")
			must.ErrorIs(t, err, ErrSandboxEscape)
		})

		t.Run("DisableArtifactInspection", func(t *testing.T) {
			ac := artifactConfig(10 * time.Second)
			sbox := New(ac, logger)

			_, taskDir := SetupDir(t)
			env := noopTaskEnv(taskDir)
			sbox.ac.DisableFilesystemIsolation = true
			sbox.ac.DisableArtifactInspection = true

			err := sbox.Get(env, artifact, "nobody")
			must.NoError(t, err)
		})
	})

	t.Run("symlink within sandbox", func(t *testing.T) {
		dir, err := os.MkdirTemp(t.TempDir(), "fake-repo")
		must.NoError(t, err, must.Sprint("failed to create local repo"))
		// create a file to link to
		f, err := os.Create(filepath.Join(dir, "test-file"))
		must.NoError(t, err, must.Sprint("could not create test file in local repo"))
		f.Close()
		// move into local repo to create relative link
		wd, err := os.Getwd()
		must.NoError(t, err, must.Sprint("cannot determine working directory"))
		must.NoError(t, os.Chdir(dir))
		must.NoError(t, os.Symlink(filepath.Base(f.Name()), "good-file"), must.Sprint("could not create symlink in local repo"))
		must.NoError(t, os.Chdir(wd))

		// now serve the repo
		srv := makeAndServeGitRepo(t, dir)

		artifact := &structs.TaskArtifact{
			RelativeDest: "local/symlink",
			GetterSource: fmt.Sprintf("git::%s/%s", srv.URL, filepath.Base(dir)),
		}

		ac := artifactConfig(10 * time.Second)
		sbox := New(ac, logger)

		_, taskDir := SetupDir(t)
		env := noopTaskEnv(taskDir)
		sbox.ac.DisableFilesystemIsolation = true

		err = sbox.Get(env, artifact, "nobody")
		must.NoError(t, err)
	})

	t.Run("ignores existing symlinks", func(t *testing.T) {
		taskDir, sbox, env := sandboxSetup()
		src, _ := servTestFile(t, "test-file")
		must.NoError(t, os.Symlink("/", filepath.Join(taskDir, "bad-file")))

		artifact := &structs.TaskArtifact{
			GetterSource: src,
			RelativeDest: "local/downloads",
		}

		err := sbox.Get(env, artifact, "nobody")
		must.NoError(t, err)

		_, err = os.Stat(filepath.Join(taskDir, "local", "downloads", "test-file"))
		must.NoError(t, err)
	})

	t.Run("properly chowns destination", func(t *testing.T) {
		taskDir, sbox, env := sandboxSetup()
		src, _ := servTestFile(t, "test-file")

		artifact := &structs.TaskArtifact{
			GetterSource: src,
			RelativeDest: "local/downloads",
			Chown:        true,
		}

		err := sbox.Get(env, artifact, "nobody")
		must.NoError(t, err)

		info, err := os.Stat(filepath.Join(taskDir, "local", "downloads"))
		must.NoError(t, err)

		uid := info.Sys().(*syscall.Stat_t).Uid
		must.Eq(t, 65534, uid) // nobody's conventional uid

		info, err = os.Stat(filepath.Join(taskDir, "local", "downloads", "test-file"))
		must.NoError(t, err)

		uid = info.Sys().(*syscall.Stat_t).Uid
		must.Eq(t, 65534, uid) // nobody's conventional uid
	})

	t.Run("when destination file exists", func(t *testing.T) {
		taskDir, sbox, env := sandboxSetup()
		src, _ := servTestFile(t, "test-file")

		testFile := filepath.Join(taskDir, "local", "downloads", "test-file")
		must.NoError(t, os.MkdirAll(filepath.Dir(testFile), 0755))
		f, err := os.OpenFile(testFile, os.O_CREATE, 0644)
		must.NoError(t, err)
		f.Write([]byte("testing"))
		f.Close()
		originalInfo, err := os.Stat(testFile)
		must.NoError(t, err)

		artifact := &structs.TaskArtifact{
			GetterSource: src,
			RelativeDest: "local/downloads",
			Chown:        true,
		}

		err = sbox.Get(env, artifact, "nobody")
		must.NoError(t, err)

		newInfo, err := os.Stat(testFile)
		must.NoError(t, err)

		must.False(t, os.SameFile(originalInfo, newInfo))
	})

	t.Run("when destination directory exists", func(t *testing.T) {
		taskDir, sbox, env := sandboxSetup()
		src, _ := servTestFile(t, "test-file")

		testFile := filepath.Join(taskDir, "local", "downloads", "testfile.txt")
		must.NoError(t, os.MkdirAll(filepath.Dir(testFile), 0755))
		f, err := os.OpenFile(testFile, os.O_CREATE, 0644)
		must.NoError(t, err)
		f.Write([]byte("testing"))
		f.Close()

		artifact := &structs.TaskArtifact{
			GetterSource: src,
			RelativeDest: "local/downloads",
			Chown:        true,
		}

		err = sbox.Get(env, artifact, "nobody")
		must.NoError(t, err)

		// check that new file exists
		_, err = os.Stat(filepath.Join(taskDir, "local", "downloads", "test-file"))
		must.NoError(t, err)

		// check that existing file still exists
		_, err = os.Stat(testFile)
		must.NoError(t, err)
	})

	t.Run("when unpacking file to an existing directory", func(t *testing.T) {
		taskDir, sbox, env := sandboxSetup()

		tarFiles := []string{
			"test.file",
			"nested/test.file",
			"other/test.file",
		}
		src, _ := servTarFile(t, tarFiles...)

		testFile := filepath.Join(taskDir, "local", "downloads", "other", "testfile.txt")
		must.NoError(t, os.MkdirAll(filepath.Dir(testFile), 0755))
		f, err := os.Create(testFile)
		must.NoError(t, err)
		f.Write([]byte("testing"))
		f.Close()

		artifact := &structs.TaskArtifact{
			GetterSource: src,
			RelativeDest: "local/downloads",
			Chown:        true,
		}

		err = sbox.Get(env, artifact, "nobody")
		must.NoError(t, err)

		// check that all unpacked files exist
		for _, tarFile := range tarFiles {
			_, err := os.Stat(filepath.Join(taskDir, "local", "downloads", tarFile))
			must.NoError(t, err)
		}

		// check existing file remains
		_, err = os.Stat(testFile)
		must.NoError(t, err)
	})
}

func servTestFile(t *testing.T, filename string) (string, *httptest.Server) {
	t.Helper()

	dir, err := os.MkdirTemp(t.TempDir(), "file")
	must.NoError(t, err)
	f, err := os.Create(filepath.Join(dir, filename))
	must.NoError(t, err)
	defer f.Close()
	f.Write([]byte(testFileContent))

	s := servDir(t, dir)
	return fmt.Sprintf("%s/%s", s.URL, filename), s
}

func servTarFile(t *testing.T, paths ...string) (string, *httptest.Server) {
	t.Helper()

	dir, err := os.MkdirTemp(t.TempDir(), "tar")
	f, err := os.Create(filepath.Join(dir, "test-compressed.tar"))
	must.NoError(t, err)
	defer f.Close()

	w := tar.NewWriter(f)
	defer w.Close()
	for _, path := range paths {
		err := w.WriteHeader(&tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(testFileContent)),
		})
		must.NoError(t, err)
		bytes, err := w.Write([]byte(testFileContent))
		must.NoError(t, err)
		must.Eq(t, len(testFileContent), bytes)
	}

	s := servDir(t, dir)
	return fmt.Sprintf("%s/test-compressed.tar", s.URL), s
}

func servDir(t *testing.T, dir string) *httptest.Server {
	t.Helper()

	fs := os.DirFS(dir)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, fs, r.URL.Path)
	}))
	t.Cleanup(s.Close)

	return s
}

func makeAndServeGitRepo(t *testing.T, repoPath string) *httptest.Server {
	t.Helper()

	wd, err := os.Getwd()
	must.NoError(t, err, must.Sprint("could not determine working directory"))
	must.NoError(t, os.Chdir(repoPath), must.Sprint("failed to change into repository directory"))
	defer func() { must.NoError(t, os.Chdir(wd), must.Sprint("failed to return to working directory")) }()

	git, err := exec.LookPath("git")
	must.NoError(t, err, must.Sprint("could not locate git executable"))

	cmd := exec.Command("git", "init", ".")
	must.NoError(t, cmd.Run(), must.Sprint("cannot init git repository"))

	cmd = exec.Command("git", "config", "user.email", "user@example.com")
	must.NoError(t, cmd.Run(), must.Sprint("cannot configure git repository"))

	cmd = exec.Command("git", "config", "user.name", "test user")
	must.NoError(t, cmd.Run(), must.Sprint("cannot configure git repository"))

	cmd = exec.Command("git", "add", "--all")
	must.NoError(t, cmd.Run(), must.Sprint("could not add files to git repository"))

	cmd = exec.Command("git", "commit", "-m", "test commit")
	must.NoError(t, cmd.Run(), must.Sprint("cannot commit git repository content"))

	handler := &cgi.Handler{
		Path: git,
		Args: []string{"http-backend"},
		Env: []string{
			"GIT_HTTP_EXPORT_ALL=true",
			fmt.Sprintf("GIT_PROJECT_ROOT=%s", filepath.Dir(repoPath)),
		},
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return srv
}
