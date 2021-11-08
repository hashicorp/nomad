package taskrunner

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	vapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
	"testing"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*vaultHook)(nil)
var _ interfaces.TaskStopHook = (*vaultHook)(nil)
var _ interfaces.ShutdownHook = (*vaultHook)(nil)

func newTestVault(t *testing.T, logger hclog.Logger) (*testutil.TestVault, vaultclient.VaultClient, error) {
	testVault := testutil.NewTestVault(t)

	vc, err := vaultclient.NewVaultClient(testVault.Config, logger, func(alloc *structs.Allocation, tasks []string, v *vapi.Client) (map[string]string, error) {
		tokens := make(map[string]string)
		for _, taskName := range tasks {
			task := alloc.LookupTask(taskName)
			tcr := vapi.TokenCreateRequest{
				Policies:  task.Vault.Policies,
				Renewable: new(bool),
			}
			*tcr.Renewable = true

			s, err := testVault.Client.Auth().Token().Create(&tcr)
			if err != nil {
				return nil, err
			}
			tokens[taskName] = s.Auth.ClientToken
		}

		return tokens, nil
	})

	return testVault, vc, err
}

func TestVaultHook_Secret(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	logger := testlog.HCLogger(t)

	testVault, vaultClient, err := newTestVault(t, logger)
	r.NoError(err)
	defer testVault.Stop()

	policy := "test"
	vaultPath := "secret/data/password"
	key := "password"
	content := "bar"

	// Write policy & secret to Vault
	sys := testVault.Client.Sys()
	r.NoError(sys.PutPolicy(policy, `
		path "secret/data/*" {
			capabilities = ["read"]
		}
	`))
	logical := testVault.Client.Logical()
	_, err = logical.Write(vaultPath, map[string]interface{}{"data": map[string]interface{}{key: content}})
	r.NoError(err)

	// Create alloc, task and taskrunner
	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "ConnectNative")
	defer cleanup()

	alloc := mock.Alloc()
	task := alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0]

	trConfig, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()
	tr, err := NewTaskRunner(trConfig)
	r.NoError(err)

	// Build the vault stanza
	secretName := "topsecret"
	vaultStanza := structs.Vault{
		Policies: []string{policy},
		Secrets: []*structs.VaultSecret{
			{Name: secretName, Path: vaultPath},
		},
	}
	task.Vault = &vaultStanza

	h := newVaultHook(&vaultHookConfig{
		vaultStanza: &vaultStanza,
		client:      vaultClient,
		lifecycle:   tr,
		updater:     tr,
		envBuilder:  tr.envBuilder,
		logger:      logger,
		alloc:       alloc,
		task:        task.Name,
	})

	request := &interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: allocDir.NewTaskDir(task.Name),
	}
	r.NoError(request.TaskDir.Build(false, nil))

	response := new(interfaces.TaskPrestartResponse)

	// Run vault client & prestart hook
	vaultClient.Start()
	defer vaultClient.Stop()
	r.NoError(h.Prestart(context.Background(), request, response))

	// Assert the secret was put into envbuilder
	env := tr.envBuilder.Build().All()
	r.Equal(env["secret."+secretName+"."+key], content)
	values, errs, err := tr.envBuilder.Build().AllValues()
	r.Empty(errs)
	r.NoError(err)
	r.Equal(values["secret"].AsValueMap()[secretName].AsValueMap()[key].AsString(), content)
}
