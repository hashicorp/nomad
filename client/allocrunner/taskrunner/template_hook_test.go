// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

func Test_templateHook_Prestart_ConsulWI(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	// Create some alloc hook resources, one with tokens and an empty one.
	defaultToken := uuid.Generate()
	hrTokens := cstructs.NewAllocHookResources()
	hrTokens.SetConsulTokens(
		map[string]map[string]*consulapi.ACLToken{
			structs.ConsulDefaultCluster: {
				fmt.Sprintf("consul_%s/web", structs.ConsulDefaultCluster): &consulapi.ACLToken{
					SecretID: defaultToken,
				},
			},
		},
	)
	hrEmpty := cstructs.NewAllocHookResources()

	tests := []struct {
		name            string
		taskConsul      *structs.Consul
		groupConsul     *structs.Consul
		hr              *cstructs.AllocHookResources
		wantErrMsg      string
		wantConsulToken string
		legacyFlow      bool
	}{
		{
			// COMPAT remove in 1.9+
			name:            "legacy flow",
			hr:              hrEmpty,
			legacyFlow:      true,
			wantConsulToken: "",
		},
		{
			name:       "task missing Consul token",
			hr:         hrEmpty,
			wantErrMsg: "not found",
		},
		{
			name:            "task without consul blocks uses default cluster",
			hr:              hrTokens,
			wantConsulToken: defaultToken,
		},
		{
			name: "task with consul block at task level",
			hr:   hrTokens,
			taskConsul: &structs.Consul{
				Cluster: structs.ConsulDefaultCluster,
			},
			wantConsulToken: defaultToken,
		},
		{
			name: "task with consul block at group level",
			hr:   hrTokens,
			groupConsul: &structs.Consul{
				Cluster: structs.ConsulDefaultCluster,
			},
			wantConsulToken: defaultToken,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mock.Alloc()

			task := a.Job.TaskGroups[0].Tasks[0]
			if !tt.legacyFlow {
				task.Identities = []*structs.WorkloadIdentity{
					{Name: fmt.Sprintf("%s_%s",
						structs.ConsulTaskIdentityNamePrefix,
						structs.ConsulDefaultCluster,
					)},
				}
			}

			clientConfig := &config.Config{Region: "global"}
			envBuilder := taskenv.NewBuilder(mock.Node(), a, task, clientConfig.Region)
			taskHooks := trtesting.NewMockTaskHooks()

			conf := &templateHookConfig{
				alloc:         a,
				logger:        logger,
				lifecycle:     taskHooks,
				events:        &trtesting.MockEmitter{},
				clientConfig:  clientConfig,
				envBuilder:    envBuilder,
				hookResources: tt.hr,
			}
			h := &templateHook{
				config:      conf,
				logger:      logger,
				managerLock: sync.Mutex{},
			}
			req := &interfaces.TaskPrestartRequest{
				Alloc:   a,
				Task:    a.Job.TaskGroups[0].Tasks[0],
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			}

			err := h.Prestart(context.Background(), req, nil)
			if tt.wantErrMsg != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tt.wantErrMsg)
			} else {
				must.NoError(t, err)
			}

			must.Eq(t, tt.wantConsulToken, h.consulToken)
		})
	}
}

func Test_templateHook_Prestart_Vault(t *testing.T) {
	ci.Parallel(t)

	secretsResp := `
{
  "data": {
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
	reqCh := make(chan any)
	defaultVaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCh <- struct{}{}
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

	testCases := []struct {
		name            string
		vault           *structs.Vault
		expectedCluster string
	}{
		{
			name: "use default cluster",
			vault: &structs.Vault{
				Cluster: structs.VaultDefaultCluster,
			},
			expectedCluster: structs.VaultDefaultCluster,
		},
		{
			name:            "use default cluster if no vault block is provided",
			vault:           nil,
			expectedCluster: structs.VaultDefaultCluster,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup alloc and task to connect to Vault cluster.
			alloc := mock.MinAlloc()
			task := alloc.Job.TaskGroups[0].Tasks[0]
			task.Vault = tc.vault

			// Setup template hook.
			taskDir := t.TempDir()
			hookConfig := &templateHookConfig{
				alloc:        alloc,
				logger:       testlog.HCLogger(t),
				lifecycle:    trtesting.NewMockTaskHooks(),
				events:       &trtesting.MockEmitter{},
				clientConfig: clientConfig,
				envBuilder:   taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region),
				templates: []*structs.Template{
					{
						EmbeddedTmpl: `{{with secret "secret/data/test"}}{{.Data.data.secret}}{{end}}`,
						ChangeMode:   structs.TemplateChangeModeNoop,
						DestPath:     path.Join(taskDir, "out.txt"),
					},
				},
			}
			hook := newTemplateHook(hookConfig)

			// Start template hook with a timeout context to ensure it exists.
			req := &interfaces.TaskPrestartRequest{
				Alloc:   alloc,
				Task:    task,
				TaskDir: &allocdir.TaskDir{Dir: taskDir},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			t.Cleanup(cancel)

			// Start in a goroutine because Prestart() blocks until first
			// render.
			hookErrCh := make(chan error)
			go func() {
				err := hook.Prestart(ctx, req, nil)
				hookErrCh <- err
			}()

			var gotRequest bool
		LOOP:
			for {
				select {
				// Register mock Vault server received a request.
				case <-reqCh:
					gotRequest = true

				// Verify test doesn't timeout.
				case <-ctx.Done():
					must.NoError(t, ctx.Err())
					return

				// Verify hook.Prestart() doesn't errors.
				case err := <-hookErrCh:
					must.NoError(t, err)
					break LOOP
				}
			}

			// Verify mock Vault server received a request.
			must.True(t, gotRequest)
		})
	}
}

// TestTemplateHook_RestoreChangeModeScript exercises change_mode=script
// behavior for a task restored after a client restart
func TestTemplateHook_RestoreChangeModeScript(t *testing.T) {

	logger := testlog.HCLogger(t)
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "foo.txt")
	must.NoError(t, os.WriteFile(destPath, []byte("original-content"), 0755))

	clientConfig := config.DefaultConfig()
	clientConfig.TemplateConfig.DisableSandbox = true

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region)

	lifecycle := trtesting.NewMockTaskHooks()
	lifecycle.SetupExecTest(117, fmt.Errorf("oh no"))
	lifecycle.HasHandle = true

	events := &trtesting.MockEmitter{}

	hook := newTemplateHook(&templateHookConfig{
		alloc:     alloc,
		logger:    logger,
		lifecycle: lifecycle,
		events:    events,
		templates: []*structs.Template{{
			DestPath:     destPath,
			EmbeddedTmpl: "changed-content",
			ChangeMode:   structs.TemplateChangeModeScript,
			ChangeScript: &structs.ChangeScript{
				Command: "echo",
				Args:    []string{"foo"},
			},
		}},
		clientConfig:  clientConfig,
		envBuilder:    envBuilder,
		hookResources: &cstructs.AllocHookResources{},
	})
	req := &interfaces.TaskPrestartRequest{
		Alloc:   alloc,
		Task:    task,
		TaskDir: &allocdir.TaskDir{Dir: tmpDir},
	}

	must.NoError(t, hook.Prestart(context.TODO(), req, nil))

	// self-test the test by making sure we really changed the template file
	out, err := os.ReadFile(destPath)
	must.NoError(t, err)
	must.Eq(t, "changed-content", string(out))

	// verify our change script executed
	gotEvents := events.Events()
	must.Len(t, 1, gotEvents)
	must.Eq(t, structs.TaskHookFailed, gotEvents[0].Type)
	must.Eq(t, "Template failed to run script echo with arguments [foo] on change: oh no. Exit code: 117",
		gotEvents[0].DisplayMessage)

}
