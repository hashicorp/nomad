// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/go-homedir"
	"github.com/shoenig/test/must"
)

func TestUtil_getURL(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		artifact *structs.TaskArtifact
		expURL   string
		expErr   *Error
	}{{
		name:     "basic http",
		artifact: &structs.TaskArtifact{GetterSource: "example.com"},
		expURL:   "example.com",
		expErr:   nil,
	}, {
		name:     "bad url",
		artifact: &structs.TaskArtifact{GetterSource: "::example.com"},
		expURL:   "",
		expErr: &Error{
			URL:         "::example.com",
			Err:         errors.New(`failed to parse source URL "::example.com": parse "::example.com": missing protocol scheme`),
			Recoverable: false,
		},
	}, {
		name: "option",
		artifact: &structs.TaskArtifact{
			GetterSource:  "git::github.com/hashicorp/nomad",
			GetterOptions: map[string]string{"sshkey": "abc123"},
		},
		expURL: "git::github.com/hashicorp/nomad?sshkey=abc123",
		expErr: nil,
	}, {
		name: "github case",
		artifact: &structs.TaskArtifact{
			GetterSource:  "git@github.com:hashicorp/nomad.git",
			GetterOptions: map[string]string{"sshkey": "abc123"},
		},
		expURL: "git@github.com:hashicorp/nomad.git?sshkey=abc123",
		expErr: nil,
	}}

	env := noopTaskEnv("/path/to/task")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getURL(env, tc.artifact)
			err2, _ := err.(*Error)
			must.Equal(t, tc.expErr, err2)
			must.Eq(t, tc.expURL, result)
		})
	}
}

func TestUtil_getDestination(t *testing.T) {
	ci.Parallel(t)

	env := noopTaskEnv("/path/to/task")
	t.Run("ok", func(t *testing.T) {
		result, err := getDestination(env, &structs.TaskArtifact{
			RelativeDest: "local/downloads",
		})
		must.NoError(t, err)
		must.Eq(t, "/path/to/task/local/downloads", result)
	})

	t.Run("escapes", func(t *testing.T) {
		result, err := getDestination(env, &structs.TaskArtifact{
			RelativeDest: "../../../../../../../etc",
		})
		must.EqError(t, err, "artifact destination path escapes alloc directory")
		must.Eq(t, "", result)
	})
}

func TestUtil_getMode(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		mode string
		exp  getter.ClientMode
	}{
		{mode: structs.GetterModeFile, exp: getter.ClientModeFile},
		{mode: structs.GetterModeDir, exp: getter.ClientModeDir},
		{mode: structs.GetterModeAny, exp: getter.ClientModeAny},
	}

	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			artifact := &structs.TaskArtifact{GetterMode: tc.mode}
			result := getMode(artifact)
			must.Eq(t, tc.exp, result)
		})
	}
}

func TestUtil_getHeaders(t *testing.T) {
	ci.Parallel(t)

	env := upTaskEnv("/path/to/task")

	t.Run("empty", func(t *testing.T) {
		result := getHeaders(env, &structs.TaskArtifact{
			GetterHeaders: nil,
		})
		must.Nil(t, result)
	})

	t.Run("replacements", func(t *testing.T) {
		result := getHeaders(env, &structs.TaskArtifact{
			GetterHeaders: map[string]string{
				"color":  "red",
				"number": "six",
			},
		})
		must.MapEq(t, map[string][]string{
			"Color":  {"RED"},
			"Number": {"SIX"},
		}, result)
	})
}

func TestUtil_getTaskDir(t *testing.T) {
	ci.Parallel(t)

	env := noopTaskEnv("/path/to/alloc/task")
	allocDir, taskDir := getWritableDirs(env)
	must.Eq(t, "/path/to/alloc", allocDir)
	must.Eq(t, "/path/to/alloc/task", taskDir)
}

func TestUtil_environment(t *testing.T) {
	// not parallel

	testutil.RequireLinux(t)

	homedir.DisableCache = true
	t.Cleanup(func() {
		homedir.DisableCache = false
	})

	t.Run("default", func(t *testing.T) {
		t.Setenv("HOME", "/test")
		result := environment("/a/b/c", "")
		must.Eq(t, []string{
			"HOME=/test",
			"PATH=/usr/local/bin:/usr/bin:/bin",
			"TMPDIR=/a/b/c/tmp",
		}, result)
	})

	t.Run("append", func(t *testing.T) {
		t.Setenv("HOME", "/test")
		t.Setenv("ONE", "1")
		t.Setenv("TWO", "2")
		result := environment("/a/b/c", "ONE,TWO")
		must.Eq(t, []string{
			"HOME=/test",
			"ONE=1",
			"PATH=/usr/local/bin:/usr/bin:/bin",
			"TMPDIR=/a/b/c/tmp",
			"TWO=2",
		}, result)
	})

	t.Run("override", func(t *testing.T) {
		t.Setenv("HOME", "/test")
		t.Setenv("PATH", "/opt/bin")
		t.Setenv("TMPDIR", "/scratch")
		result := environment("/a/b/c", "PATH,TMPDIR")
		must.Eq(t, []string{
			"HOME=/test",
			"PATH=/opt/bin",
			"TMPDIR=/scratch",
		}, result)
	})

	t.Run("missing", func(t *testing.T) {
		t.Setenv("HOME", "/test")
		result := environment("/a/b/c", "DOES_NOT_EXIST")
		must.Eq(t, []string{
			"DOES_NOT_EXIST=",
			"HOME=/test",
			"PATH=/usr/local/bin:/usr/bin:/bin",
			"TMPDIR=/a/b/c/tmp",
		}, result)
	})

	t.Run("homeless non-root", func(t *testing.T) {
		testutil.RequireNonRoot(t)

		// assert we fallback via go-homdir ...
		userHome, err := homedir.Dir()
		must.NoError(t, err)

		// ... when HOME env var is not set, as is the case in some systemd setups
		t.Setenv("HOME", "")

		result := environment("/a/b/c", "")
		must.Eq(t, []string{
			fmt.Sprintf("HOME=%s", userHome),
			"PATH=/usr/local/bin:/usr/bin:/bin",
			"TMPDIR=/a/b/c/tmp",
		}, result)
	})

	t.Run("homeless root", func(t *testing.T) {
		testutil.RequireRoot(t)

		t.Setenv("HOME", "/root") // fake running as full root

		// assert we fallback via go-homdir ...
		userHome, err := homedir.Dir()
		must.NoError(t, err)

		// ... when HOME env var is not set, as is the case in some systemd setups
		t.Setenv("HOME", "")

		result := environment("/a/b/c", "")
		must.Eq(t, []string{
			fmt.Sprintf("HOME=%s", userHome),
			"PATH=/usr/local/bin:/usr/bin:/bin",
			"TMPDIR=/a/b/c/tmp",
		}, result)
	})
}
