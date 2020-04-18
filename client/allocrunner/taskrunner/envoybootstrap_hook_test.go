// +build !windows
// todo(shoenig): Once Connect is supported on Windows, we'll need to make this
//  set of tests work there too.

package taskrunner

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	consultest "github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

var _ interfaces.TaskPrestartHook = (*envoyBootstrapHook)(nil)

func writeTmp(t *testing.T, s string, fm os.FileMode) string {
	dir, err := ioutil.TempDir("", "envoy-")
	require.NoError(t, err)

	fPath := filepath.Join(dir, sidsTokenFile)
	err = ioutil.WriteFile(fPath, []byte(s), fm)
	require.NoError(t, err)

	return dir
}

func TestEnvoyBootstrapHook_maybeLoadSIToken(t *testing.T) {
	t.Parallel()

	// This test fails when running as root because the test case for checking
	// the error condition when the file is unreadable fails (root can read the
	// file even though the permissions are set to 0200).
	if unix.Geteuid() == 0 {
		t.Skip("test only works as non-root")
	}

	t.Run("file does not exist", func(t *testing.T) {
		h := newEnvoyBootstrapHook(&envoyBootstrapHookConfig{logger: testlog.HCLogger(t)})
		cfg, err := h.maybeLoadSIToken("task1", "/does/not/exist")
		require.NoError(t, err) // absence of token is not an error
		require.Equal(t, "", cfg)
	})

	t.Run("load token from file", func(t *testing.T) {
		token := uuid.Generate()
		f := writeTmp(t, token, 0440)
		defer cleanupDir(t, f)

		h := newEnvoyBootstrapHook(&envoyBootstrapHookConfig{logger: testlog.HCLogger(t)})
		cfg, err := h.maybeLoadSIToken("task1", f)
		require.NoError(t, err)
		require.Equal(t, token, cfg)
	})

	t.Run("file is unreadable", func(t *testing.T) {
		token := uuid.Generate()
		f := writeTmp(t, token, 0200)
		defer cleanupDir(t, f)

		h := newEnvoyBootstrapHook(&envoyBootstrapHookConfig{logger: testlog.HCLogger(t)})
		cfg, err := h.maybeLoadSIToken("task1", f)
		require.Error(t, err)
		require.False(t, os.IsNotExist(err))
		require.Equal(t, "", cfg)
	})
}

func TestEnvoyBootstrapHook_decodeTriState(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", decodeTriState(nil))
	require.Equal(t, "true", decodeTriState(helper.BoolToPtr(true)))
	require.Equal(t, "false", decodeTriState(helper.BoolToPtr(false)))
}

var (
	consulPlainConfig = envoyBootstrapConsulConfig{
		HTTPAddr: "2.2.2.2",
	}

	consulTLSConfig = envoyBootstrapConsulConfig{
		HTTPAddr:  "2.2.2.2",            // arg
		Auth:      "user:password",      // env
		SSL:       "true",               // env
		VerifySSL: "true",               // env
		CAFile:    "/etc/tls/ca-file",   // arg
		CertFile:  "/etc/tls/cert-file", // arg
		KeyFile:   "/etc/tls/key-file",  // arg
	}
)

func TestEnvoyBootstrapHook_envoyBootstrapArgs(t *testing.T) {
	t.Parallel()

	t.Run("excluding SI token", func(t *testing.T) {
		ebArgs := envoyBootstrapArgs{
			sidecarFor:     "s1",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulPlainConfig,
			envoyAdminBind: "localhost:3333",
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "localhost:3333",
			"-bootstrap",
			"-sidecar-for", "s1",
		}, result)
	})

	t.Run("including SI token", func(t *testing.T) {
		token := uuid.Generate()
		ebArgs := envoyBootstrapArgs{
			sidecarFor:     "s1",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulPlainConfig,
			envoyAdminBind: "localhost:3333",
			siToken:        token,
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "localhost:3333",
			"-bootstrap",
			"-sidecar-for", "s1",
			"-token", token,
		}, result)
	})

	t.Run("including certificates", func(t *testing.T) {
		ebArgs := envoyBootstrapArgs{
			sidecarFor:     "s1",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulTLSConfig,
			envoyAdminBind: "localhost:3333",
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "localhost:3333",
			"-bootstrap",
			"-sidecar-for", "s1",
			"-ca-file", "/etc/tls/ca-file",
			"-client-cert", "/etc/tls/cert-file",
			"-client-key", "/etc/tls/key-file",
		}, result)
	})
}

func TestEnvoyBootstrapHook_envoyBootstrapEnv(t *testing.T) {
	t.Parallel()

	environment := []string{"foo=bar", "baz=1"}

	t.Run("plain consul config", func(t *testing.T) {
		require.Equal(t, []string{
			"foo=bar", "baz=1",
		}, envoyBootstrapArgs{
			sidecarFor:     "s1",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulPlainConfig,
			envoyAdminBind: "localhost:3333",
		}.env(environment))
	})

	t.Run("tls consul config", func(t *testing.T) {
		require.Equal(t, []string{
			"foo=bar", "baz=1",
			"CONSUL_HTTP_AUTH=user:password",
			"CONSUL_HTTP_SSL=true",
			"CONSUL_HTTP_SSL_VERIFY=true",
		}, envoyBootstrapArgs{
			sidecarFor:     "s1",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulTLSConfig,
			envoyAdminBind: "localhost:3333",
		}.env(environment))
	})
}

// dig through envoy config to look for consul token
type envoyConfig struct {
	DynamicResources struct {
		ADSConfig struct {
			GRPCServices struct {
				InitialMetadata []struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"initial_metadata"`
			} `json:"grpc_services"`
		} `json:"ads_config"`
	} `json:"dynamic_resources"`
}

// TestEnvoyBootstrapHook_with_SI_token asserts the bootstrap file written for
// Envoy contains a Consul SI token.
func TestEnvoyBootstrapHook_with_SI_token(t *testing.T) {
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
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, logger))
	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	// Insert service identity token in the secrets directory
	token := uuid.Generate()
	siTokenFile := filepath.Join(req.TaskDir.SecretsDir, sidsTokenFile)
	err = ioutil.WriteFile(siTokenFile, []byte(token), 0440)
	require.NoError(t, err)

	resp := &interfaces.TaskPrestartResponse{}

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), req, resp))

	// Assert it is Done
	require.True(t, resp.Done)

	// Ensure the default path matches
	env := map[string]string{
		taskenv.SecretsDir: req.TaskDir.SecretsDir,
	}
	f, err := os.Open(args.ReplaceEnv(structs.EnvoyBootstrapPath, env))
	require.NoError(t, err)
	defer f.Close()

	// Assert bootstrap configuration is valid json
	var out envoyConfig
	require.NoError(t, json.NewDecoder(f).Decode(&out))

	// Assert the SI token got set
	key := out.DynamicResources.ADSConfig.GRPCServices.InitialMetadata[0].Key
	value := out.DynamicResources.ADSConfig.GRPCServices.InitialMetadata[0].Value
	require.Equal(t, "x-consul-token", key)
	require.Equal(t, token, value)
}

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
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, logger))
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
	var out envoyConfig
	require.NoError(t, json.NewDecoder(f).Decode(&out))

	// Assert no SI token got set
	key := out.DynamicResources.ADSConfig.GRPCServices.InitialMetadata[0].Key
	value := out.DynamicResources.ADSConfig.GRPCServices.InitialMetadata[0].Value
	require.Equal(t, "x-consul-token", key)
	require.Equal(t, "", value)
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
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: "http://127.0.0.2:1",
	}, logger))
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
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: testconsul.HTTPAddr,
	}, logger))
	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	resp := &interfaces.TaskPrestartResponse{}

	// Run the hook
	err = h.Prestart(context.Background(), req, resp)
	require.EqualError(t, err, "error creating bootstrap configuration for Connect proxy sidecar: exit status 1")
	require.True(t, structs.IsRecoverable(err))

	// Assert it is not Done
	require.False(t, resp.Done)

	// Assert no file was written
	_, err = os.Open(filepath.Join(req.TaskDir.SecretsDir, "envoy_bootstrap.json"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}
