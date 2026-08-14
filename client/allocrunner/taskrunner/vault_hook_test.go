// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
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
	"github.com/shoenig/test"
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
		must.Wait(t, wait.ContinualSuccess(wait.Attempts(10), wait.BoolFunc(func() bool {
			return len(client.Calls) == 1
		})))
	})

	t.Run("overrides role with task vault block role", func(t *testing.T) {
		widMgr := widmgr.NewMockIdentityManager()
		widMgr.SetIdentity(
			structs.WIHandle{IdentityName: "vault_default", WorkloadType: 0, WorkloadIdentifier: "t"},
			&structs.SignedWorkloadIdentity{},
		)

		client := vaultclient.NewMockVaultClient()
		// This mock will only accept `Role: "test-role"`. Any other role will fail
		client.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{Role: "test-role"}).Return(
			"testToken", false, 0, nil,
		)

		hook := setupTestVaultHook(t, &vaultHookConfig{widmgr: widMgr}, client)
		hook.task.Vault.Role = "test-role" // use "test-role"

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
		must.NoError(t, err) // Will error if a different role is passed
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

func TestVaultHook_handleRenewalFailure(t *testing.T) {
	ci.Parallel(t)

	widMgr := widmgr.NewMockIdentityManager()
	widMgr.SetIdentity(
		structs.WIHandle{IdentityName: "vault_default",
			WorkloadType: 0, WorkloadIdentifier: "t"},
		&structs.SignedWorkloadIdentity{},
	)
	updater := &vaultTokenUpdaterMock{}

	clientOk := vaultclient.NewMockVaultClient()
	clientOk.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).
		Return("testToken", true, 5, nil)

	clientErr := vaultclient.NewMockVaultClient()
	clientErr.On("DeriveTokenWithJWT", t.Context(), vaultclient.JWTLoginRequest{}).
		Return("", false, 0, errors.New("oops"))

	testCases := []struct {
		name        string
		vaultBlock  *structs.Vault
		vaultClient *vaultclient.MockVaultClient

		expectErrMsg        string
		verifyTaskLifecycle func(*testing.T, *trtesting.MockTaskHooks)
	}{
		{
			name: "change mode signal",
			vaultBlock: &structs.Vault{
				Cluster:      structs.VaultDefaultCluster,
				ChangeMode:   structs.VaultChangeModeSignal,
				ChangeSignal: "SIGTERM",
			},
			vaultClient: clientOk,
			verifyTaskLifecycle: func(t *testing.T, h *trtesting.MockTaskHooks) {
				signals := h.Signals()
				must.Len(t, 1, signals, must.Sprint("expected 1 signal"))
				test.Eq(t, "SIGTERM", signals[0])
				restarts := h.Restarts()
				test.Eq(t, 0, restarts, test.Sprint("expected no restart"))
				test.Nil(t, h.KillEvent(), test.Sprint("expected no kill"))
			},
		},
		{
			name: "change mode signal refresh error",
			vaultBlock: &structs.Vault{
				Cluster:      structs.VaultDefaultCluster,
				ChangeMode:   structs.VaultChangeModeSignal,
				ChangeSignal: "SIGTERM",
			},
			vaultClient:  clientErr,
			expectErrMsg: "failed to derive Vault token for identity vault_default: oops",
			verifyTaskLifecycle: func(t *testing.T, h *trtesting.MockTaskHooks) {
				signals := h.Signals()
				test.Len(t, 0, signals, test.Sprint("expected no signal"))
				restarts := h.Restarts()
				test.Eq(t, 0, restarts, test.Sprint("expected no restart"))
				test.NotNil(t, h.KillEvent(), test.Sprint("expected kill"))
			},
		},
		{
			name: "change mode restart",
			vaultBlock: &structs.Vault{
				Cluster:    structs.VaultDefaultCluster,
				ChangeMode: structs.VaultChangeModeRestart,
			},
			vaultClient: clientOk,
			verifyTaskLifecycle: func(t *testing.T, h *trtesting.MockTaskHooks) {
				signals := h.Signals()
				test.Len(t, 0, signals, test.Sprint("expected no signal"))
				restarts := h.Restarts()
				test.Eq(t, 1, restarts, test.Sprint("expected 1 restart"))
				test.Nil(t, h.KillEvent(), test.Sprint("expected no kill"))
			},
		},
		{
			name: "change mode noop",
			vaultBlock: &structs.Vault{
				Cluster:    structs.VaultDefaultCluster,
				ChangeMode: structs.VaultChangeModeNoop,
			},
			vaultClient: clientOk,
			verifyTaskLifecycle: func(t *testing.T, h *trtesting.MockTaskHooks) {
				signals := h.Signals()
				test.Len(t, 0, signals, test.Sprint("expected no signal"))
				restarts := h.Restarts()
				test.Eq(t, 0, restarts, test.Sprint("expected no restart"))
				test.Nil(t, h.KillEvent(), test.Sprint("expected no kill"))
			},
		},
		{
			name: "change mode noop refresh error",
			vaultBlock: &structs.Vault{
				Cluster:    structs.VaultDefaultCluster,
				ChangeMode: structs.VaultChangeModeNoop,
			},
			vaultClient:  clientErr,
			expectErrMsg: "failed to derive Vault token for identity vault_default: oops",
			verifyTaskLifecycle: func(t *testing.T, h *trtesting.MockTaskHooks) {
				signals := h.Signals()
				test.Len(t, 0, signals, test.Sprint("expected no signal"))
				restarts := h.Restarts()
				test.Eq(t, 0, restarts, test.Sprint("expected no restart"))
				test.NotNil(t, h.KillEvent(), test.Sprint("expected kill"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			hook := setupTestVaultHook(t, &vaultHookConfig{
				vaultBlock: tc.vaultBlock,
				widmgr:     widMgr,
				updater:    updater},
				tc.vaultClient)

			// required to simulate a previous PreStart running
			hook.client, _ = hook.clientFunc("default")
			hook.vaultConfig = hook.vaultConfigsFunc(hook.logger)["default"]
			hook.secretsDirTokenPath = filepath.Join(t.TempDir(), vaultTokenFile)
			hook.privateDirTokenPath = filepath.Join(t.TempDir(), vaultTokenFile)

			tok, lease, err := hook.handleRenewalFailure(ctx)

			if tc.expectErrMsg == "" {
				must.NoError(t, err)
				must.Eq(t, "testToken", tok)
				must.Eq(t, time.Duration(time.Second*5), lease)
				updater = (hook.updater).(*vaultTokenUpdaterMock)
				token := updater.currentToken
				must.Eq(t, "testToken", token)
			} else {
				must.EqError(t, err, tc.expectErrMsg)
				must.Eq(t, "", tok)
				must.Eq(t, 0, lease)
			}

			tc.verifyTaskLifecycle(t, (hook.lifecycle).(*trtesting.MockTaskHooks))
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

	config.taskCtx = t.Context()

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
