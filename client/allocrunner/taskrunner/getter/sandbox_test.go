// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
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
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

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

	// Create a temporary directory directly so the repos
	// don't end up being found improperly
	tdir, err := os.MkdirTemp("", "nomad-test")
	must.NoError(t, err, must.Sprint("failed to create top level local repo directory"))

	t.Run("symlink escaped sandbox", func(t *testing.T) {
		dir, err := os.MkdirTemp(tdir, "fake-repo")
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
		dir, err := os.MkdirTemp(tdir, "fake-repo")
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
