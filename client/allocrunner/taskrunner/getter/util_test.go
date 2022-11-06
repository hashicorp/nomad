package getter

import (
	"errors"
	"testing"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestUtil_getURL(t *testing.T) {
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
	env := upTaskEnv("/path/to/task")

	t.Run("empty", func(t *testing.T) {
		result := getHeaders(env, &structs.TaskArtifact{
			GetterHeaders: nil,
		})
		must.Nil(t, result)
	})

	t.Run("replacments", func(t *testing.T) {
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
	env := noopTaskEnv("/path/to/task")
	result := getTaskDir(env)
	must.Eq(t, "/path/to/task", result)
}

func TestUtil_minimalVars(t *testing.T) {
	result := minimalVars("/path/to/task")
	must.Eq(t, []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"TMPDIR=/path/to/task/tmp",
		"TMP=/path/to/task/tmp",
	}, result)
}
