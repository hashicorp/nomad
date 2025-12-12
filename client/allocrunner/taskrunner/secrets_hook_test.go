// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/bufconndialer"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

func TestSecretsHook_Prestart_Nomad(t *testing.T) {
	ci.Parallel(t)

	t.Run("nomad provider successfully renders valid secrets", func(t *testing.T) {
		secretsResp := `
		{
		  "CreateIndex": 812,
		  "CreateTime": 1750782609539170600,
		  "Items": {
		    "key2": "value2",
		    "key1": "value1"
		  },
		  "ModifyIndex": 812,
		  "ModifyTime": 1750782609539170600,
		  "Namespace": "default",
		  "Path": "testnomadvar"
		}
		`
		count := 0 // CT expects a nomad index header that increments, or else it continues polling
		nomadServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Nomad-Index", strconv.Itoa(count))
			fmt.Fprintln(w, secretsResp)
			count += 1
		}))
		t.Cleanup(nomadServer.Close)

		l, d := bufconndialer.New()
		nomadServer.Listener = l

		nomadServer.Start()

		clientConfig := config.DefaultConfig()
		clientConfig.TemplateDialer = d
		clientConfig.TemplateConfig.DisableSandbox = true

		taskDir := t.TempDir()
		alloc := mock.MinAlloc()
		task := alloc.Job.TaskGroups[0].Tasks[0]

		taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region)
		conf := &secretsHookConfig{
			logger:       testlog.HCLogger(t),
			lifecycle:    trtesting.NewMockTaskHooks(),
			events:       &trtesting.MockEmitter{},
			clientConfig: clientConfig,
			envBuilder:   taskEnv,
		}
		secretHook := newSecretsHook(conf, []*structs.Secret{
			{
				Name:     "test_secret",
				Provider: "nomad",
				Path:     "testnomadvar",
				Config: map[string]any{
					"namespace": "default",
				},
			},
			{
				Name:     "test_secret1",
				Provider: "nomad",
				Path:     "testnomadvar1",
				Config: map[string]any{
					"namespace": "default",
				},
			},
		})

		req := &interfaces.TaskPrestartRequest{
			Alloc:   alloc,
			Task:    task,
			TaskDir: &allocdir.TaskDir{Dir: taskDir, SecretsDir: taskDir},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)

		err := secretHook.Prestart(ctx, req, &interfaces.TaskPrestartResponse{})
		must.NoError(t, err)

		expected := map[string]string{
			"secret.test_secret.key1":  "value1",
			"secret.test_secret.key2":  "value2",
			"secret.test_secret1.key1": "value1",
			"secret.test_secret1.key2": "value2",
		}
		must.Eq(t, expected, taskEnv.Build().TaskSecrets)
	})

	t.Run("returns early if context is cancelled", func(t *testing.T) {

		secretsResp := `
		{
		  "CreateIndex": 812,
		  "CreateTime": 1750782609539170600,
		  "Items": {
		    "key2": "value2",
		    "key1": "value1"
		  },
		  "ModifyIndex": 812,
		  "ModifyTime": 1750782609539170600,
		  "Namespace": "default",
		  "Path": "testnomadvar"
		}
		`
		count := 0 // CT expects a nomad index header that increments, or else it continues polling
		nomadServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Nomad-Index", strconv.Itoa(count))
			fmt.Fprintln(w, secretsResp)
			count += 1
		}))
		t.Cleanup(nomadServer.Close)

		l, d := bufconndialer.New()
		nomadServer.Listener = l

		nomadServer.Start()

		clientConfig := config.DefaultConfig()
		clientConfig.TemplateDialer = d
		clientConfig.TemplateConfig.DisableSandbox = true

		taskDir := t.TempDir()
		alloc := mock.MinAlloc()
		task := alloc.Job.TaskGroups[0].Tasks[0]

		taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region)
		conf := &secretsHookConfig{
			logger:       testlog.HCLogger(t),
			lifecycle:    trtesting.NewMockTaskHooks(),
			events:       &trtesting.MockEmitter{},
			clientConfig: clientConfig,
			envBuilder:   taskEnv,
		}
		secretHook := newSecretsHook(conf, []*structs.Secret{
			{
				Name:     "test_secret",
				Provider: "nomad",
				Path:     "testnomadvar",
				Config: map[string]any{
					"namespace": "default",
				},
			},
		})

		req := &interfaces.TaskPrestartRequest{
			Alloc:   alloc,
			Task:    task,
			TaskDir: &allocdir.TaskDir{Dir: taskDir, SecretsDir: taskDir},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cancel() // cancel context to simulate task being stopped

		err := secretHook.Prestart(ctx, req, &interfaces.TaskPrestartResponse{})
		must.NoError(t, err)

		expected := map[string]string{}
		must.Eq(t, expected, taskEnv.Build().TaskSecrets)
	})

	t.Run("errors when failure building secret providers", func(t *testing.T) {
		clientConfig := config.DefaultConfig()

		taskDir := t.TempDir()
		alloc := mock.MinAlloc()
		task := alloc.Job.TaskGroups[0].Tasks[0]

		taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region)
		conf := &secretsHookConfig{
			logger:       testlog.HCLogger(t),
			lifecycle:    trtesting.NewMockTaskHooks(),
			events:       &trtesting.MockEmitter{},
			clientConfig: clientConfig,
			envBuilder:   taskEnv,
		}

		// give an invalid secret, in this case a nomad secret with bad namespace
		secretHook := newSecretsHook(conf, []*structs.Secret{
			{
				Name:     "test_secret",
				Provider: "nomad",
				Path:     "testnomadvar",
				Config: map[string]any{
					"namespace": 123,
				},
			},
		})

		req := &interfaces.TaskPrestartRequest{
			Alloc:   alloc,
			Task:    task,
			TaskDir: &allocdir.TaskDir{Dir: taskDir, SecretsDir: taskDir},
		}

		// Prestart should error and return after building secrets
		err := secretHook.Prestart(context.Background(), req, nil)
		must.Error(t, err)

		expected := map[string]string{}
		must.Eq(t, expected, taskEnv.Build().TaskSecrets)
	})
}

func TestSecretsHook_Prestart_Vault(t *testing.T) {
	ci.Parallel(t)

	secretsResp := `
{
  "Data": {
    "data": {
      "secret": "secret"
    },
    "metadata": {
      "created_time": "2023-10-18T15:58:29.65137Z",
      "custom_metadata": null,
      "deletion_time": "",
      "destroyed": false,
      "version": 1
    }
  }
}`

	// Start test server to simulate Vault cluster responses.
	// reqCh := make(chan any)
	defaultVaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, secretsResp)
	}))
	t.Cleanup(defaultVaultServer.Close)

	// Setup client with Vault config.
	clientConfig := config.DefaultConfig()
	clientConfig.TemplateConfig.DisableSandbox = true
	clientConfig.VaultConfigs = map[string]*structsc.VaultConfig{
		structs.VaultDefaultCluster: {
			Name:    structs.VaultDefaultCluster,
			Enabled: pointer.Of(true),
			Addr:    defaultVaultServer.URL,
		},
	}

	taskDir := t.TempDir()
	alloc := mock.MinAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region)
	conf := &secretsHookConfig{
		logger:       testlog.HCLogger(t),
		lifecycle:    trtesting.NewMockTaskHooks(),
		events:       &trtesting.MockEmitter{},
		clientConfig: clientConfig,
		envBuilder:   taskEnv,
	}
	secretHook := newSecretsHook(conf, []*structs.Secret{
		{
			Name:     "test_secret",
			Provider: "vault",
			Path:     "/test/path",
			Config: map[string]any{
				"engine": "kv_v2",
			},
		},
		{
			Name:     "test_secret1",
			Provider: "vault",
			Path:     "/test/path1",
			Config: map[string]any{
				"engine": "kv_v2",
			},
		},
	})

	// Start template hook with a timeout context to ensure it exists.
	req := &interfaces.TaskPrestartRequest{
		Alloc:   alloc,
		Task:    task,
		TaskDir: &allocdir.TaskDir{Dir: taskDir, SecretsDir: taskDir},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	err := secretHook.Prestart(ctx, req, &interfaces.TaskPrestartResponse{})
	must.NoError(t, err)

	exp := map[string]string{
		"secret.test_secret.secret":  "secret",
		"secret.test_secret1.secret": "secret",
	}

	must.Eq(t, exp, taskEnv.Build().TaskSecrets)
}

func TestSecretsHook_Prestart_Plugin(t *testing.T) {
	basePlugin := `#!/bin/bash
if [ "$1" = "fingerprint" ]; then
    cat <<EOF
{
  "type": "secrets",
  "version": "0.0.1"
}
EOF
elif [ "$1" = "fetch" ]; then
    cat <<EOF
{
  "result": {
	%s
  }
}
EOF
fi`

	t.Run("sets plugin environment correctly", func(t *testing.T) {
		clientConfig := config.DefaultConfig()
		clientConfig.CommonPluginDir = t.TempDir()

		pluginDir := filepath.Join(clientConfig.CommonPluginDir, "secrets")
		err := os.MkdirAll(pluginDir, 0755)
		must.NoError(t, err)

		pluginPath := filepath.Join(pluginDir, "test")
		testPlugin := fmt.Sprintf(basePlugin, `
				"jobID": "${NOMAD_JOB_ID}",
				"namespace": "${NOMAD_NAMESPACE}"`)
		err = os.WriteFile(pluginPath, []byte(testPlugin), 0755)
		must.NoError(t, err)

		taskDir := t.TempDir()
		alloc := mock.MinAlloc()
		task := alloc.Job.TaskGroups[0].Tasks[0]

		taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region)
		conf := &secretsHookConfig{
			logger:         testlog.HCLogger(t),
			lifecycle:      trtesting.NewMockTaskHooks(),
			events:         &trtesting.MockEmitter{},
			clientConfig:   clientConfig,
			envBuilder:     taskEnv,
			nomadNamespace: "test-namespace",
			jobId:          "test-jobid",
		}
		secretHook := newSecretsHook(conf, []*structs.Secret{
			{
				Name:     "test_secret0",
				Provider: "test",
				Path:     "/test/path",
				Env: map[string]string{
					"NOMAD_NAMESPACE": "incorrect",
					"NOMAD_JOB_ID":    "also-incorrect",
				},
			},
			{
				Name:     "test_secret1",
				Provider: "test",
				Path:     "/test/path",
			},
		})

		// Start template hook with a timeout context to ensure it exists.
		req := &interfaces.TaskPrestartRequest{
			Alloc:   alloc,
			Task:    task,
			TaskDir: &allocdir.TaskDir{Dir: taskDir, SecretsDir: taskDir},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)

		err = secretHook.Prestart(ctx, req, &interfaces.TaskPrestartResponse{})
		must.NoError(t, err)

		exp := map[string]string{
			"secret.test_secret0.jobID":     "test-jobid",
			"secret.test_secret0.namespace": "test-namespace",
			"secret.test_secret1.jobID":     "test-jobid",
			"secret.test_secret1.namespace": "test-namespace",
		}

		must.Eq(t, exp, taskEnv.Build().TaskSecrets)
	})
}
