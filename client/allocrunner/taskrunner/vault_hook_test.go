// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	nmock "github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/mock"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*vaultHook)(nil)
var _ interfaces.TaskStopHook = (*vaultHook)(nil)
var _ interfaces.ShutdownHook = (*vaultHook)(nil)

func TestVaultHook_Prestart(t *testing.T) {

	t.Run("derives a token and renews it", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		// return a lease time of 0, so it is quickly renewed
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"testToken", true, 0, nil,
		)
		client.On("Renew", mock.Anything, "testToken", 0).Return(time.Minute, nil)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)

		must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
			if slices.ContainsFunc(client.Calls, func(m mock.Call) bool {
				return m.Method == "Renew"
			}) {
				return nil
			}
			return errors.New("Has not called both derive and renew yet")
		})))
	})

	t.Run("does not renew non-renewable token", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"testToken", false, 0, nil,
		)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)
		hook.allowTokenExpiration = false // explicitly set this to false

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)
		must.True(t, hook.allowTokenExpiration)
	})

	t.Run("overrides role with task vault block role", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{Role: "test-role"}).Return(
			"testToken", false, 0, nil,
		)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)
		hook.task.Vault.Role = "test-role"

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)
	})

	t.Run("reads existing token from private dir", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		updater := &vaultTokenUpdaterMock{}
		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr, updater: updater}, client)

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		os.WriteFile(filepath.Join(req.TaskDir.PrivateDir, vaultTokenFile), []byte("testToken"), 0600)

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)
		must.Len(t, 0, client.Calls)
		must.Eq(t, updater.currentToken, "testToken")
	})

	t.Run("reads existing token from secret dir", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		updater := &vaultTokenUpdaterMock{}
		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr, updater: updater}, client)

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		os.WriteFile(filepath.Join(req.TaskDir.SecretsDir, vaultTokenFile), []byte("testToken"), 0600)

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)
		must.Len(t, 0, client.Calls)
		must.Eq(t, updater.currentToken, "testToken")
	})

	t.Run("does not write to file when disabled", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"testToken", false, 0, nil,
		)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)
		hook.task.Vault.DisableFile = true

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)

		_, err = os.Stat(filepath.Join(req.TaskDir.SecretsDir, vaultTokenFile))
		must.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("retries if DeriveToken returns recoverable error", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"", false, 0, structs.NewRecoverableError(errors.New("try again!"), true),
		).Times(1)

		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"testToken", false, 0, nil,
		)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)
	})

	t.Run("exits with error if DeriveToken returns unrecoverable error", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"", false, 0, structs.NewRecoverableError(errors.New("go away"), false),
		).Times(1)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.Error(t, err)
	})

	t.Run("retries if Renew returns recoverable error", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"testToken", true, 0, nil,
		)

		client.On("Renew", mock.Anything, "testToken", 0).Return(
			time.Minute,
			structs.NewRecoverableError(errors.New("try again!"), true),
		).Times(1)

		client.On("Renew", mock.Anything, "testToken", 0).Return(time.Minute, nil).Times(1)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)

		must.Wait(t, wait.InitialSuccess(wait.Timeout(6*time.Second), wait.ErrorFunc(func() error {
			if len(client.Calls) == 3 {
				return nil
			}
			return errors.New("has not called renew twice")
		})))
	})

	t.Run("trigger lifecycle if Renew returns unrecoverable error", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).Return(
			"testToken", true, 0, nil,
		)
		client.On("Renew", mock.Anything, "testToken", 0).Return(
			time.Minute,
			errors.New("permission denied"),
		)

		mockLifecycle := trtesting.NewMockTaskHooks()
		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)
		hook.lifecycle = mockLifecycle
		hook.task.Vault.ChangeMode = structs.VaultChangeModeRestart

		var resp interfaces.TaskPrestartResponse
		req := &interfaces.TaskPrestartRequest{
			TaskEnv: taskenv.NewEmptyTaskEnv(),
			TaskDir: &allocdir.TaskDir{
				SecretsDir: t.TempDir(),
				PrivateDir: t.TempDir(),
			},
			Task: hook.task,
		}

		err := hook.Prestart(t.Context(), req, &resp)
		must.NoError(t, err)
		must.Wait(t, wait.InitialSuccess(wait.Timeout(1*time.Second), wait.ErrorFunc(func() error {
			if mockLifecycle.KillEvent() != nil {
				return nil
			}
			return errors.New("test")
		})))
	})
}

func TestVaultHook_handleRenewal(t *testing.T) {
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
			vaultClient := vaultclient.NewMockVaultClient()

			hook := setupTestVaultHook(t, &vaultHookConfig{vaultBlock: tc.vaultBlock}, vaultClient)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			t.Cleanup(cancel)

			hook.handleRenewal(ctx, "secret")

			// Fetch derived token.
			updater := (hook.updater).(*vaultTokenUpdaterMock)
			token := updater.currentToken
			must.NotEq(t, "", token)

			// Verify expected lifecycle events happen.
			must.Wait(t, wait.InitialSuccess(
				wait.ErrorFunc(func() error {
					return tc.verifyTaskLifecycle((hook.lifecycle).(*trtesting.MockTaskHooks))
				}),
				wait.Timeout(3*time.Second),
				wait.Gap(1000*time.Millisecond),
			))
		})
	}
}

// vaultTokenUpdaterMock is a mock of the vaultTokenUpdateHandler interface.
type vaultTokenUpdaterMock struct {
	currentToken string
}

func (v *vaultTokenUpdaterMock) updatedVaultToken(token string) {
	v.currentToken = token
}

func setupTestVaultHook(t *testing.T, config *vaultHookConfig, client *vaultclient.MockVaultClient) *vaultHook {
	t.Helper()

	if config == nil {
		config = &vaultHookConfig{}
	}

	job := nmock.MinJob()
	if config.alloc == nil {
		config.alloc = nmock.MinAlloc()
		config.alloc.Job = job
	}
	if config.task == nil {
		config.task = job.TaskGroups[0].Tasks[0]
		config.task.Identities = []*structs.WorkloadIdentity{
			{Name: "vault_default"},
		}
		config.task.Vault = &structs.Vault{
			Cluster:    structs.VaultDefaultCluster,
			ChangeMode: structs.VaultChangeModeNoop,
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
			return client, nil
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
		config.widmgr = widmgr.NewMockIdentityManager()
	}

	return newVaultHook(config)
}
