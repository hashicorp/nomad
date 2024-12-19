// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*vaultHook)(nil)
var _ interfaces.TaskStopHook = (*vaultHook)(nil)
var _ interfaces.ShutdownHook = (*vaultHook)(nil)

// vaultTokenUpdaterMock is a mock of the vaultTokenUpdateHandler interface.
type vaultTokenUpdaterMock struct {
	currentToken string
}

func (v *vaultTokenUpdaterMock) updatedVaultToken(token string) {
	v.currentToken = token
}

func setupTestVaultHook(t *testing.T, config *vaultHookConfig) *vaultHook {
	t.Helper()

	if config == nil {
		config = &vaultHookConfig{}
	}

	job := mock.MinJob()
	if config.alloc == nil {
		config.alloc = mock.MinAlloc()
		config.alloc.Job = job
	}
	if config.task == nil {
		config.task = job.TaskGroups[0].Tasks[0]
		config.task.Identities = []*structs.WorkloadIdentity{
			{Name: "vault_default"},
		}
		config.task.Vault = &structs.Vault{
			Cluster: structs.VaultDefaultCluster,
		}

		if config.vaultBlock != nil {
			config.task.Identities[0].Name = config.vaultBlock.IdentityName()
			config.task.Vault = config.vaultBlock
		}
	}
	if config.vaultBlock == nil {
		config.vaultBlock = config.task.Vault
	}
	if config.vaultConfigsFunc == nil {
		config.vaultConfigsFunc = func(hclog.Logger) map[string]*sconfig.VaultConfig {
			return map[string]*sconfig.VaultConfig{
				"default": sconfig.DefaultVaultConfig(),
			}
		}
	}
	if config.clientFunc == nil {
		config.clientFunc = func(cluster string) (vaultclient.VaultClient, error) {
			return vaultclient.NewMockVaultClient(cluster)
		}
	}
	if config.logger == nil {
		config.logger = testlog.HCLogger(t)
	}
	if config.events == nil {
		config.events = &trtesting.MockEmitter{}
	}
	if config.lifecycle == nil {
		config.lifecycle = trtesting.NewMockTaskHooks()
	}
	if config.updater == nil {
		config.updater = &vaultTokenUpdaterMock{}
	}
	if config.widmgr == nil {
		db := cstate.NewMemDB(config.logger)
		signer := widmgr.NewMockWIDSigner(config.task.Identities)
		envBuilder := taskenv.NewBuilder(mock.Node(), config.alloc, nil, "global")
		config.widmgr = widmgr.NewWIDMgr(signer, config.alloc, db, config.logger, envBuilder)
		err := config.widmgr.Run()
		must.NoError(t, err)
	}

	return newVaultHook(config)
}

func TestTaskRunner_VaultHook(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name               string
		task               *structs.Task
		configs            map[string]*sconfig.VaultConfig
		configNonrenewable bool
		expectRole         string
		expectLegacy       bool
		expectNoRenew      bool
	}{
		{
			name: "legacy flow",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster: structs.VaultDefaultCluster,
				},
			},
			expectLegacy: true,
		},
		{
			name: "jwt flow",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster: structs.VaultDefaultCluster,
				},
				Identities: []*structs.WorkloadIdentity{
					{Name: "vault_default"},
				},
			},
		},
		{
			name: "jwt flow with role",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster: structs.VaultDefaultCluster,
					Role:    "task-role",
				},
				Identities: []*structs.WorkloadIdentity{
					{Name: "vault_default"},
				},
			},
			configs: map[string]*sconfig.VaultConfig{
				"default": {
					Role: "client-role",
				},
			},
			expectRole: "task-role",
		},
		{
			name: "jwt flow with role from client",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster: structs.VaultDefaultCluster,
				},
				Identities: []*structs.WorkloadIdentity{
					{Name: "vault_default"},
				},
			},
			configs: map[string]*sconfig.VaultConfig{
				"default": {
					Role: "client-role",
				},
			},
			expectRole: "client-role",
		},
		{
			name: "jwt flow with role from client and non-default cluster",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster: "prod",
				},
				Identities: []*structs.WorkloadIdentity{
					{Name: "vault_prod"},
				},
			},
			configs: map[string]*sconfig.VaultConfig{
				"default": {
					Role: "client-role",
				},
				"prod": {
					Role: "client-prod-role",
				},
			},
			expectRole: "client-prod-role",
		},
		{
			name: "disable file",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster:     structs.VaultDefaultCluster,
					DisableFile: true,
				},
				Identities: []*structs.WorkloadIdentity{
					{Name: "vault_default"},
				},
			},
		},
		{
			name: "job requests no renewal",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster:              structs.VaultDefaultCluster,
					AllowTokenExpiration: true,
				},
				Identities: []*structs.WorkloadIdentity{
					{Name: "vault_default"},
				},
			},
			expectNoRenew: true,
		},
		{
			name: "tokens are not renewable",
			task: &structs.Task{
				Vault: &structs.Vault{
					Cluster: structs.VaultDefaultCluster,
				},
				Identities: []*structs.WorkloadIdentity{
					{Name: "vault_default"},
				},
			},
			configNonrenewable: true,
			expectNoRenew:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := mock.MinAlloc()
			alloc.Job.TaskGroups[0].Tasks[0] = tc.task

			hookConfig := &vaultHookConfig{
				task:  tc.task,
				alloc: alloc,
				vaultConfigsFunc: func(hclog.Logger) map[string]*sconfig.VaultConfig {
					if tc.configs != nil {
						return tc.configs
					}
					return map[string]*sconfig.VaultConfig{
						"default": sconfig.DefaultVaultConfig(),
					}
				},
			}

			if tc.configNonrenewable {
				hookConfig.clientFunc = func(cluster string) (vaultclient.VaultClient, error) {
					client := &vaultclient.MockVaultClient{}
					client.SetRenewable(false)
					return client, nil
				}
			}

			hook := setupTestVaultHook(t, hookConfig)

			// Ensure Prestart() returns within a reasonable time.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			t.Cleanup(cancel)

			req := &interfaces.TaskPrestartRequest{
				TaskEnv: taskenv.NewEmptyTaskEnv(),
				TaskDir: &allocdir.TaskDir{
					SecretsDir: t.TempDir(),
					PrivateDir: t.TempDir(),
				},
				Task: tc.task,
			}
			var resp interfaces.TaskPrestartResponse

			err := hook.Prestart(ctx, req, &resp)
			must.NoError(t, err)
			must.NoError(t, ctx.Err())

			// Token must have been derived.
			var token string
			client := hook.client.(*vaultclient.MockVaultClient)
			if tc.expectLegacy {
				tokens := client.LegacyTokens()
				must.MapLen(t, 1, tokens)
				token = tokens[tc.task.Name]
			} else {
				tokens := client.JWTTokens()
				must.MapLen(t, 1, tokens)

				swid, err := hook.widmgr.Get(structs.WIHandle{
					IdentityName:       tc.task.Vault.IdentityName(),
					WorkloadIdentifier: tc.task.Name,
					WorkloadType:       structs.WorkloadTypeTask,
				})
				must.NoError(t, err)
				token = tokens[swid.JWT]
			}
			must.NotEq(t, "", token)

			// Token must be derived with correct role.
			//
			// MockVaultClient generates random UUIDv4 tokens, but append the
			// role when requested.
			if tc.expectRole != "" {
				must.StrHasSuffix(t, tc.expectRole, token)
			} else {
				must.UUIDv4(t, token)
			}

			// Token must be set in token updater.
			updater := (hook.updater).(*vaultTokenUpdaterMock)
			must.Eq(t, token, updater.currentToken)

			// Token must be written to disk.
			tokenFile, err := os.ReadFile(hook.privateDirTokenPath)
			must.NoError(t, err)
			must.Eq(t, updater.currentToken, string(tokenFile))

			if !tc.task.Vault.DisableFile {
				tokenFile, err := os.ReadFile(hook.secretsDirTokenPath)
				must.NoError(t, err)
				must.Eq(t, updater.currentToken, string(tokenFile))
			} else {
				_, err = os.ReadFile(hook.secretsDirTokenPath)
				must.ErrorIs(t, err, os.ErrNotExist)
			}

			// Token must be set for renewal.
			if tc.expectNoRenew {
				must.MapEmpty(t, client.RenewTokens())
			} else {
				must.MapLen(t, 1, client.RenewTokens())
				must.NotNil(t, client.RenewTokens()[updater.currentToken])
			}

			// PrestartDone must be false so we can recover tokens.
			// firstRun is used to prevent multiple executions.
			must.False(t, resp.Done)
			must.False(t, hook.firstRun)

			// Stop renewal when hook stops.
			err = hook.Stop(ctx, nil, nil)
			must.NoError(t, err)
			must.Wait(t, wait.InitialSuccess(
				wait.ErrorFunc(func() error {
					tokens := client.StoppedTokens()

					if tc.expectNoRenew {
						if len(tokens) != 0 {
							return fmt.Errorf("expected no stopped tokens when renewal is disabled, got %d", len(tokens))
						}
						return nil
					}

					if len(tokens) != 1 {
						return fmt.Errorf("expected stopped tokens to be %d, got %d", 1, len(tokens))
					}
					got := tokens[0]
					expect := updater.currentToken
					if got != expect {
						return fmt.Errorf("expected stopped token to be %s, got %s", expect, got)
					}
					return nil
				}),
				wait.Timeout(5*time.Second),
				wait.Gap(100*time.Millisecond),
			))
		})
	}
}

func TestTaskRunner_VaultHook_recover(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		setupReq func() (*interfaces.TaskPrestartRequest, error)
	}{
		{
			name: "recover from secrets dir",
			setupReq: func() (*interfaces.TaskPrestartRequest, error) {
				// Write token to secrets dir.
				secretsDirPath := t.TempDir()
				err := os.WriteFile(filepath.Join(secretsDirPath, vaultTokenFile), []byte("much secret"), 0666)
				if err != nil {
					return nil, err
				}

				req := &interfaces.TaskPrestartRequest{
					TaskEnv: taskenv.NewEmptyTaskEnv(),
					TaskDir: &allocdir.TaskDir{
						SecretsDir: secretsDirPath,
						PrivateDir: t.TempDir(),
					},
				}
				return req, nil
			},
		},
		{
			name: "recover from private dir",
			setupReq: func() (*interfaces.TaskPrestartRequest, error) {
				// Write token to private dir.
				privateDirPath := t.TempDir()
				err := os.WriteFile(filepath.Join(privateDirPath, vaultTokenFile), []byte("much secret"), 0666)
				if err != nil {
					return nil, err
				}

				req := &interfaces.TaskPrestartRequest{
					TaskEnv: taskenv.NewEmptyTaskEnv(),
					TaskDir: &allocdir.TaskDir{
						SecretsDir: t.TempDir(),
						PrivateDir: privateDirPath,
					},
				}
				return req, nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hook := setupTestVaultHook(t, nil)

			req, err := tc.setupReq()
			must.NoError(t, err)
			req.Task = hook.task

			// Ensure Prestart() returns in a reasonable time.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			t.Cleanup(cancel)

			var resp interfaces.TaskPrestartResponse
			err = hook.Prestart(ctx, req, &resp)
			must.NoError(t, err)
			must.NoError(t, ctx.Err())

			// Verify token was recovered and not derived.
			client := hook.client.(*vaultclient.MockVaultClient)
			must.MapLen(t, 0, client.JWTTokens())
			must.MapLen(t, 0, client.LegacyTokens())
		})
	}
}

func TestTaskRunner_VaultHook_deriveError(t *testing.T) {
	ci.Parallel(t)

	t.Run("unrecoverable error", func(t *testing.T) {
		vaultClient, _ := vaultclient.NewMockVaultClient("")
		mockVaultClient := vaultClient.(*vaultclient.MockVaultClient)

		hook := setupTestVaultHook(t, &vaultHookConfig{
			clientFunc: func(string) (vaultclient.VaultClient, error) {
				return mockVaultClient, nil
			},
		})
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}
		var resp interfaces.TaskPrestartResponse

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)

		// Set unrecoverable error.
		mockVaultClient.SetDeriveTokenWithJWTFn(
			func(_ context.Context, _ vaultclient.JWTLoginRequest) (string, bool, error) {
				// Cancel the context to simulate the task being killed.
				cancel()
				return "", false, structs.NewRecoverableError(errors.New("unrecoverable test error"), false)
			})

		err := hook.Prestart(ctx, req, &resp)
		must.NoError(t, err)

		// Verify task is killed because of unrecoverable error.
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				killEv := (hook.lifecycle.(*trtesting.MockTaskHooks)).KillEvent()
				if killEv == nil {
					return errors.New("missing kill event")
				}
				return nil
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))
		killEv := (hook.lifecycle.(*trtesting.MockTaskHooks)).KillEvent()
		must.StrContains(t, killEv.DisplayMessage, "unrecoverable test error")
	})

	t.Run("recoverable error", func(t *testing.T) {
		vaultClient, _ := vaultclient.NewMockVaultClient("")
		mockVaultClient := vaultClient.(*vaultclient.MockVaultClient)

		hook := setupTestVaultHook(t, &vaultHookConfig{
			clientFunc: func(string) (vaultclient.VaultClient, error) {
				return mockVaultClient, nil
			},
		})
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}
		var resp interfaces.TaskPrestartResponse

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		// Set recoverable error.
		mockVaultClient.SetDeriveTokenWithJWTFn(
			func(_ context.Context, _ vaultclient.JWTLoginRequest) (string, bool, error) {
				return "", false, structs.NewRecoverableError(errors.New("recoverable test error"), true)
			})

		go func() {
			// Wait a bit for the first error then fix token renewal.
			time.Sleep(time.Second)
			mockVaultClient.SetDeriveTokenWithJWTFn(
				func(_ context.Context, _ vaultclient.JWTLoginRequest) (string, bool, error) {
					return "secret", true, nil
				})

		}()
		err := hook.Prestart(ctx, req, &resp)
		must.NoError(t, err)
		must.NoError(t, ctx.Err())

		// Verify retry happened and token was derived.
		updater := (hook.updater).(*vaultTokenUpdaterMock)
		must.Eq(t, "secret", updater.currentToken)
	})

	t.Run("renew request failed", func(t *testing.T) {
		vaultClient, _ := vaultclient.NewMockVaultClient("")
		mockVaultClient := vaultClient.(*vaultclient.MockVaultClient)

		hook := setupTestVaultHook(t, &vaultHookConfig{
			clientFunc: func(string) (vaultclient.VaultClient, error) {
				return mockVaultClient, nil
			},
		})
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}
		var resp interfaces.TaskPrestartResponse

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		// Derive predictable token and fail renew request.
		mockVaultClient.SetDeriveTokenWithJWTFn(
			func(_ context.Context, _ vaultclient.JWTLoginRequest) (string, bool, error) {
				return "secret", true, nil
			})
		mockVaultClient.SetRenewTokenError("secret", errors.New("test error"))

		go func() {
			// Wait a bit for the renew error then fix token renewal.
			time.Sleep(10 * time.Millisecond)
			mockVaultClient.SetRenewTokenError("secret", nil)

		}()
		err := hook.Prestart(ctx, req, &resp)
		must.NoError(t, err)
		must.NoError(t, ctx.Err())

		// Verify retry happened and token was derived.
		updater := (hook.updater).(*vaultTokenUpdaterMock)
		must.Eq(t, "secret", updater.currentToken)
	})
}

func TestTaskRunner_VaultHook_tokenRenewalFail(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		vaultBlock          *structs.Vault
		verifyTaskLifecycle func(*trtesting.MockTaskHooks) error
	}{
		{
			name: "change mode signal",
			vaultBlock: &structs.Vault{
				Cluster:      structs.VaultDefaultCluster,
				ChangeMode:   structs.VaultChangeModeSignal,
				ChangeSignal: "SIGTERM",
			},
			verifyTaskLifecycle: func(h *trtesting.MockTaskHooks) error {
				signals := h.Signals()
				if len(signals) != 1 {
					return fmt.Errorf("expected 1 signal, got %d", len(signals))
				}
				if signals[0] != "SIGTERM" {
					return fmt.Errorf("expected signal to be SIGTERM, got %s", signals[0])
				}
				return nil
			},
		},
		{
			name: "change mode restart",
			vaultBlock: &structs.Vault{
				Cluster:    structs.VaultDefaultCluster,
				ChangeMode: structs.VaultChangeModeRestart,
			},
			verifyTaskLifecycle: func(h *trtesting.MockTaskHooks) error {
				restarts := h.Restarts()
				if restarts != 1 {
					return fmt.Errorf("expected 1 restart, got %d", restarts)
				}
				return nil
			},
		},
		{
			name: "change mode noop",
			vaultBlock: &structs.Vault{
				Cluster:    structs.VaultDefaultCluster,
				ChangeMode: structs.VaultChangeModeNoop,
			},
			verifyTaskLifecycle: func(h *trtesting.MockTaskHooks) error {
				restarts := h.Restarts()
				if restarts != 0 {
					return fmt.Errorf("expected 0 restarts, got %d", restarts)
				}

				signals := h.Signals()
				if len(signals) != 0 {
					return fmt.Errorf("expected 0 signals, got %d", len(signals))
				}

				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vaultClient, _ := vaultclient.NewMockVaultClient("")
			mockVaultClient := vaultClient.(*vaultclient.MockVaultClient)

			hook := setupTestVaultHook(t, &vaultHookConfig{
				vaultBlock: tc.vaultBlock,
				clientFunc: func(string) (vaultclient.VaultClient, error) {
					return mockVaultClient, nil
				},
			})

			req := &interfaces.TaskPrestartRequest{
				TaskEnv: taskenv.NewEmptyTaskEnv(),
				TaskDir: &allocdir.TaskDir{
					SecretsDir: t.TempDir(),
					PrivateDir: t.TempDir(),
				},
				Task: hook.task,
			}
			var resp interfaces.TaskPrestartResponse

			// Ensure Prestart() returns within a reasonable time.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			t.Cleanup(cancel)

			err := hook.Prestart(ctx, req, &resp)
			must.NoError(t, err)

			// Fetch derived token.
			updater := (hook.updater).(*vaultTokenUpdaterMock)
			token := updater.currentToken
			must.NotEq(t, "", token)

			// Fetch renewal token error channel.
			renewErrCh := mockVaultClient.RenewTokenErrCh(token)
			must.NotNil(t, renewErrCh)

			// Emit renewal error.
			renewErrCh <- errors.New("renew error")

			// Verify expected lifecycle events happen.
			must.Wait(t, wait.InitialSuccess(
				wait.ErrorFunc(func() error {
					return tc.verifyTaskLifecycle((hook.lifecycle).(*trtesting.MockTaskHooks))
				}),
				wait.Timeout(3*time.Second),
				wait.Gap(100*time.Millisecond),
			))
		})
	}
}
