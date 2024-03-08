// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"sync/atomic"
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
				fmt.Sprintf("consul_%s", structs.ConsulDefaultCluster): &consulapi.ACLToken{
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
			name:            "legecy flow",
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
				config:       conf,
				logger:       logger,
				managerLock:  sync.Mutex{},
				driverHandle: nil,
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

func Test_templateHook_Prestart_VaultFail(t *testing.T) {
	ci.Parallel(t)

	// Start test server to simulate Vault cluster responses.
	reqCh := make(chan any, 10)
	vaultServer := mockVaultServer(t, reqCh)
	t.Cleanup(vaultServer.Close)

	// Setup client with Vault config.
	clientConfig := config.DefaultConfig()
	clientConfig.TemplateConfig.DisableSandbox = true
	clientConfig.TemplateConfig.VaultRetry = &config.RetryConfig{
		Attempts:   pointer.Of(1),
		Backoff:    pointer.Of(100 * time.Millisecond),
		MaxBackoff: pointer.Of(100 * time.Millisecond),
	}

	clientConfig.VaultConfigs = map[string]*structsc.VaultConfig{
		structs.VaultDefaultCluster: {
			Name:    structs.VaultDefaultCluster,
			Enabled: pointer.Of(true),
			Addr:    vaultServer.URL,
		},
	}

	testCases := []struct {
		name      string
		expectErr string
	}{
		{
			name:      "exhaust retries on 403",
			expectErr: "foo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Setup template hook.
			alloc := mock.MinAlloc()
			task := alloc.Job.TaskGroups[0].Tasks[0]
			taskDir := t.TempDir()
			taskLifecycleHooks := trtesting.NewMockTaskHooks()

			hookConfig := &templateHookConfig{
				alloc:        alloc,
				logger:       testlog.HCLogger(t),
				lifecycle:    taskLifecycleHooks,
				events:       &trtesting.MockEmitter{},
				clientConfig: clientConfig,
				envBuilder:   taskenv.NewBuilder(mock.Node(), alloc, task, clientConfig.Region),
				templates: []*structs.Template{
					{
						EmbeddedTmpl: `
{{with secret "secret/data/test1"}}{{.Data.data.secret}}{{end}}
{{with secret "secret/data/test2"}}{{.Data.data.secret}}{{end}}
{{with secret "secret/data/test3"}}{{.Data.data.secret}}{{end}}
`,
						ChangeMode: structs.TemplateChangeModeNoop,
						DestPath:   path.Join(taskDir, "out.txt"),
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

			killCtx, killCancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(killCancel)

			testCtx, testCancel := context.WithTimeout(context.Background(), time.Second*3)
			t.Cleanup(testCancel)

			start := time.Now()

			// Start in a goroutine because Prestart() blocks until first
			// render.
			hookErrCh := make(chan error, 1)
			go func() {
				err := hook.Prestart(killCtx, req, nil)
				t.Logf("%v hook.Prestart done!", time.Now().Sub(start))
				hookErrCh <- err
			}()

			stopCh := make(chan error, 1)

			killTaskAndStop := func() {
				killCancel()

				// note: template_hook.Stop doesn't respect context
				err := hook.Stop(context.TODO(),
					&interfaces.TaskStopRequest{}, &interfaces.TaskStopResponse{})
				stopCh <- err
			}

			var gotRequests int
			var gotKill bool
			var gotStop bool
			var timedOut bool
		LOOP:
			for {
				select {
				case <-reqCh:
					gotRequests++ // record the number of Vault requests we send
					if gotRequests == 1 {
						go killTaskAndStop() // must be async so we can get Stop hook result

					}

				case <-taskLifecycleHooks.KillCh:
					gotKill = true
					t.Logf("%v KILL recv!", time.Now().Sub(start))
					go killTaskAndStop() // must be async so we can get Stop hook result

				case err := <-stopCh:
					t.Logf("%v hook.Stop done!", time.Now().Sub(start))
					must.NoError(t, err, must.Sprint("expected no error from Stop hook"))
					gotStop = true
					break LOOP

				case <-testCtx.Done():
					t.Logf("%v test timeout!", time.Now().Sub(start))
					timedOut = true
					must.NoError(t, testCtx.Err(), must.Sprint("test timed out"))
					return

				case err := <-hookErrCh:
					t.Logf("%v hookErrCh recv! (%v)", time.Now().Sub(start), err)
					must.NoError(t, err, must.Sprint("expected no error from Prestart hook"))
				}
			}
			t.Logf("%v all done!", time.Now().Sub(start))

			must.False(t, timedOut, must.Sprintf("timed out!"))
			must.True(t, gotStop, must.Sprint("expected stop"))
			must.False(t, gotKill, must.Sprint("expected kill"))
			//			must.Eq(t, 3, gotRequests, must.Sprint("expected requests to use up retries"))
		})
	}
}

func mockVaultServer(t *testing.T, reqCh chan any) *httptest.Server {
	t.Helper()

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

	preflightResp := `
{
  "request_id": "5667af97-0fa4-d36f-92aa-1f256560c69a",
  "lease_id": "",
  "renewable": false,
  "lease_duration": 0,
  "data": {
    "accessor": "kv_4b3570ef",
    "config": {
      "default_lease_ttl": 0,
      "force_no_cache": false,
      "max_lease_ttl": 0
    },
    "deprecation_status": "supported",
    "description": "key/value secret storage",
    "external_entropy_access": false,
    "local": false,
    "options": {
      "version": "2"
    },
    "path": "secret/",
    "plugin_version": "",
    "running_plugin_version": "v0.15.0+builtin",
    "running_sha256": "",
    "seal_wrap": false,
    "type": "kv",
    "uuid": "f046f163-8359-f3bf-7bac-29bee259073f"
  },
  "wrap_info": null,
  "warnings": null,
  "auth": null
}
`
	// Start test server to simulate Vault cluster responses.
	reqNum := atomic.Uintptr{}
	vaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("GET %s", r.URL.Path)

		if strings.HasPrefix(r.URL.Path, "/v1/sys") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(preflightResp))
		} else {
			reqNum.Add(1)
			//			if reqNum%3 == 0 {
			if strings.HasSuffix(r.URL.Path, "test3") {
				time.Sleep(500 * time.Millisecond)
				http.Error(w, "Forbidden", http.StatusForbidden)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(secretsResp))
			}
		}

		reqCh <- struct{}{}

	}))

	return vaultServer
}
