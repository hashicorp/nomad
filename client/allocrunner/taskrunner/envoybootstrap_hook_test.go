package taskrunner

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	consultest "github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ interfaces.TaskPrestartHook = (*envoyBootstrapHook)(nil)

// TestTaskRunner_EnvoyBootstrapHook_Prestart asserts the EnvoyBootstrapHook
// creates Envoy's bootstrap.json configuration based on Connect proxy sidecars
// registered for the task.
func TestTaskRunner_EnvoyBootstrapHook_Ok(t *testing.T) {
	t.Parallel()
	testutil.RequireConsul(t)

	testconsul, err := consultest.NewTestServerConfig(func(c *consultest.TestServerConfig) {
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		}
	})
	if err != nil {
		t.Fatalf("error starting test consul server: %v", err)
	}
	defer testconsul.Stop()

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
			IP:   "10.0.0.1",
			DynamicPorts: []structs.Port{
				{
					Label: "connect-proxy-foo",
					Value: 9999,
					To:    9999,
				},
			},
		},
	}
	tg := alloc.Job.TaskGroups[0]
	tg.Services = []*structs.Service{
		{
			Name:      "foo",
			PortLabel: "9999", // Just need a valid port, nothing will bind to it
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		},
	}
	sidecarTask := &structs.Task{
		Name: "sidecar",
		Kind: "connect-proxy:foo",
	}
	tg.Tasks = append(tg.Tasks, sidecarTask)

	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap")
	defer cleanup()

	// Register Group Services
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testconsul.HTTPAddr
	consulAPIClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	consulClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), logger, true)
	go consulClient.Run()
	defer consulClient.Shutdown()
	require.NoError(t, consulClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

	// Run Connect bootstrap Hook
	h := newEnvoyBootstrapHook(alloc, testconsul.HTTPAddr, logger)
	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	resp := &interfaces.TaskPrestartResponse{}

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), req, resp))

	// Assert it is Done
	require.True(t, resp.Done)

	require.NotNil(t, resp.Env)
	require.Equal(t, "localhost:19001", resp.Env[envoyAdminBindEnvPrefix+"foo"])

	// Ensure the default path matches
	env := map[string]string{
		taskenv.SecretsDir: req.TaskDir.SecretsDir,
	}
	f, err := os.Open(args.ReplaceEnv(structs.EnvoyBootstrapPath, env))
	require.NoError(t, err)
	defer f.Close()

	// Assert bootstrap configuration is valid json
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(f).Decode(&out))
}

// TestTaskRunner_EnvoyBootstrapHook_Noop asserts that the Envoy bootstrap hook
// is a noop for non-Connect proxy sidecar tasks.
func TestTaskRunner_EnvoyBootstrapHook_Noop(t *testing.T) {
	t.Parallel()
	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap")
	defer cleanup()

	alloc := mock.Alloc()
	task := alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0]

	// Run Envoy bootstrap Hook. Use invalid Consul address as it should
	// not get hit.
	h := newEnvoyBootstrapHook(alloc, "http://127.0.0.2:1", logger)
	req := &interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: allocDir.NewTaskDir(task.Name),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	resp := &interfaces.TaskPrestartResponse{}

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), req, resp))

	// Assert it is Done
	require.True(t, resp.Done)

	// Assert no file was written
	_, err := os.Open(filepath.Join(req.TaskDir.SecretsDir, "envoy_bootstrap.json"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

// TestTaskRunner_EnvoyBootstrapHook_RecoverableError asserts the Envoy
// bootstrap hook returns a Recoverable error if the bootstrap command runs but
// fails.
func TestTaskRunner_EnvoyBootstrapHook_RecoverableError(t *testing.T) {
	t.Parallel()
	testutil.RequireConsul(t)

	testconsul, err := consultest.NewTestServerConfig(func(c *consultest.TestServerConfig) {
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		}
	})
	if err != nil {
		t.Fatalf("error starting test consul server: %v", err)
	}
	defer testconsul.Stop()

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
			IP:   "10.0.0.1",
			DynamicPorts: []structs.Port{
				{
					Label: "connect-proxy-foo",
					Value: 9999,
					To:    9999,
				},
			},
		},
	}
	tg := alloc.Job.TaskGroups[0]
	tg.Services = []*structs.Service{
		{
			Name:      "foo",
			PortLabel: "9999", // Just need a valid port, nothing will bind to it
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		},
	}
	sidecarTask := &structs.Task{
		Name: "sidecar",
		Kind: "connect-proxy:foo",
	}
	tg.Tasks = append(tg.Tasks, sidecarTask)

	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap")
	defer cleanup()

	// Unlike the successful test above, do NOT register the group services
	// yet. This should cause a recoverable error similar to if Consul was
	// not running.

	// Run Connect bootstrap Hook
	h := newEnvoyBootstrapHook(alloc, testconsul.HTTPAddr, logger)
	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	resp := &interfaces.TaskPrestartResponse{}

	// Run the hook
	err = h.Prestart(context.Background(), req, resp)
	require.Error(t, err)
	require.True(t, structs.IsRecoverable(err))

	// Assert it is not Done
	require.False(t, resp.Done)

	// Assert no file was written
	_, err = os.Open(filepath.Join(req.TaskDir.SecretsDir, "envoy_bootstrap.json"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}
