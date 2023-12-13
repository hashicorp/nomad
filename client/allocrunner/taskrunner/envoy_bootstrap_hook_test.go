// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows
// +build !windows

// todo(shoenig): Once Connect is supported on Windows, we'll need to make this
//  set of tests work there too.

package taskrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

var _ interfaces.TaskPrestartHook = (*envoyBootstrapHook)(nil)

const (
	// consulNamespace is empty string in OSS, because Consul OSS does not like
	// having even the default namespace set.
	consulNamespace = ""
)

func writeTmp(t *testing.T, s string, fm os.FileMode) string {
	dir := t.TempDir()

	fPath := filepath.Join(dir, sidsTokenFile)
	err := os.WriteFile(fPath, []byte(s), fm)
	require.NoError(t, err)

	return dir
}

func TestEnvoyBootstrapHook_maybeLoadSIToken(t *testing.T) {
	ci.Parallel(t)

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

		h := newEnvoyBootstrapHook(&envoyBootstrapHookConfig{logger: testlog.HCLogger(t)})
		cfg, err := h.maybeLoadSIToken("task1", f)
		require.NoError(t, err)
		require.Equal(t, token, cfg)
	})

	t.Run("file is unreadable", func(t *testing.T) {
		token := uuid.Generate()
		f := writeTmp(t, token, 0200)

		h := newEnvoyBootstrapHook(&envoyBootstrapHookConfig{logger: testlog.HCLogger(t)})
		cfg, err := h.maybeLoadSIToken("task1", f)
		require.Error(t, err)
		require.False(t, os.IsNotExist(err))
		require.Equal(t, "", cfg)
	})
}

func TestEnvoyBootstrapHook_decodeTriState(t *testing.T) {
	ci.Parallel(t)

	require.Equal(t, "", decodeTriState(nil))
	require.Equal(t, "true", decodeTriState(pointer.Of(true)))
	require.Equal(t, "false", decodeTriState(pointer.Of(false)))
}

var (
	consulPlainConfig = consulTransportConfig{
		HTTPAddr: "2.2.2.2",
	}

	consulTLSConfig = consulTransportConfig{
		HTTPAddr:   "2.2.2.2",               // arg
		Auth:       "user:password",         // env
		SSL:        "true",                  // env
		VerifySSL:  "true",                  // env
		GRPCCAFile: "/etc/tls/grpc-ca-file", // arg
		CAFile:     "/etc/tls/ca-file",      // arg
		CertFile:   "/etc/tls/cert-file",    // arg
		KeyFile:    "/etc/tls/key-file",     // arg
	}
)

func TestEnvoyBootstrapHook_envoyBootstrapArgs(t *testing.T) {
	ci.Parallel(t)

	t.Run("excluding SI token", func(t *testing.T) {
		ebArgs := envoyBootstrapArgs{
			proxyID:        "s1-sidecar-proxy",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulPlainConfig,
			envoyAdminBind: "127.0.0.2:19000",
			envoyReadyBind: "127.0.0.1:19100",
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "127.0.0.2:19000",
			"-address", "127.0.0.1:19100",
			"-proxy-id", "s1-sidecar-proxy",
			"-bootstrap",
		}, result)
	})

	t.Run("including SI token", func(t *testing.T) {
		token := uuid.Generate()
		ebArgs := envoyBootstrapArgs{
			proxyID:        "s1-sidecar-proxy",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulPlainConfig,
			envoyAdminBind: "127.0.0.2:19000",
			envoyReadyBind: "127.0.0.1:19100",
			siToken:        token,
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "127.0.0.2:19000",
			"-address", "127.0.0.1:19100",
			"-proxy-id", "s1-sidecar-proxy",
			"-bootstrap",
			"-token", token,
		}, result)
	})

	t.Run("including certificates", func(t *testing.T) {
		ebArgs := envoyBootstrapArgs{
			proxyID:        "s1-sidecar-proxy",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulTLSConfig,
			envoyAdminBind: "127.0.0.2:19000",
			envoyReadyBind: "127.0.0.1:19100",
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "127.0.0.2:19000",
			"-address", "127.0.0.1:19100",
			"-proxy-id", "s1-sidecar-proxy",
			"-bootstrap",
			"-grpc-ca-file", "/etc/tls/grpc-ca-file",
			"-ca-file", "/etc/tls/ca-file",
			"-client-cert", "/etc/tls/cert-file",
			"-client-key", "/etc/tls/key-file",
		}, result)
	})

	t.Run("ingress gateway", func(t *testing.T) {
		ebArgs := envoyBootstrapArgs{
			consulConfig:   consulPlainConfig,
			grpcAddr:       "1.1.1.1",
			envoyAdminBind: "127.0.0.2:19000",
			envoyReadyBind: "127.0.0.1:19100",
			gateway:        "my-ingress-gateway",
			proxyID:        "_nomad-task-803cb569-881c-b0d8-9222-360bcc33157e-group-ig-ig-8080",
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "127.0.0.2:19000",
			"-address", "127.0.0.1:19100",
			"-proxy-id", "_nomad-task-803cb569-881c-b0d8-9222-360bcc33157e-group-ig-ig-8080",
			"-bootstrap",
			"-gateway", "my-ingress-gateway",
		}, result)
	})

	t.Run("mesh gateway", func(t *testing.T) {
		ebArgs := envoyBootstrapArgs{
			consulConfig:   consulPlainConfig,
			grpcAddr:       "1.1.1.1",
			envoyAdminBind: "127.0.0.2:19000",
			envoyReadyBind: "127.0.0.1:19100",
			gateway:        "my-mesh-gateway",
			proxyID:        "_nomad-task-803cb569-881c-b0d8-9222-360bcc33157e-group-mesh-mesh-8080",
		}
		result := ebArgs.args()
		require.Equal(t, []string{"connect", "envoy",
			"-grpc-addr", "1.1.1.1",
			"-http-addr", "2.2.2.2",
			"-admin-bind", "127.0.0.2:19000",
			"-address", "127.0.0.1:19100",
			"-proxy-id", "_nomad-task-803cb569-881c-b0d8-9222-360bcc33157e-group-mesh-mesh-8080",
			"-bootstrap",
			"-gateway", "my-mesh-gateway",
		}, result)
	})
}

func TestEnvoyBootstrapHook_envoyBootstrapEnv(t *testing.T) {
	ci.Parallel(t)

	environment := []string{"foo=bar", "baz=1"}

	t.Run("plain consul config", func(t *testing.T) {
		require.Equal(t, []string{
			"foo=bar", "baz=1",
		}, envoyBootstrapArgs{
			proxyID:        "s1-sidecar-proxy",
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
			proxyID:        "s1-sidecar-proxy",
			grpcAddr:       "1.1.1.1",
			consulConfig:   consulTLSConfig,
			envoyAdminBind: "localhost:3333",
		}.env(environment))
	})
}

// envoyConfig is used to unmarshal an envoy bootstrap configuration file, so that
// we can inspect the contents in tests.
type envoyConfig struct {
	Admin struct {
		Address struct {
			SocketAddress struct {
				Address string `json:"address"`
				Port    int    `json:"port_value"`
			} `json:"socket_address"`
		} `json:"address"`
	} `json:"admin"`
	Node struct {
		Cluster  string `json:"cluster"`
		ID       string `json:"id"`
		Metadata struct {
			Namespace string `json:"namespace"`
			Version   string `json:"envoy_version"`
		}
	}
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
	ci.Parallel(t)
	testutil.RequireConsul(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	alloc := mock.ConnectAlloc()
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

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap", alloc.ID)
	defer cleanup()

	// Register Group Services
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testConsul.HTTPAddr
	consulAPIClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	namespacesClient := agentconsul.NewNamespacesClient(consulAPIClient.Namespaces(), consulAPIClient.Agent())

	consulClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), namespacesClient, logger, true)
	go consulClient.Run()
	defer consulClient.Shutdown()
	require.NoError(t, consulClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

	// Run Connect bootstrap Hook
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, consulNamespace, logger))
	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
		TaskEnv: taskenv.NewEmptyTaskEnv(),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	// Insert service identity token in the secrets directory
	token := uuid.Generate()
	siTokenFile := filepath.Join(req.TaskDir.SecretsDir, sidsTokenFile)
	err = os.WriteFile(siTokenFile, []byte(token), 0440)
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

// TestTaskRunner_EnvoyBootstrapHook_sidecar_ok asserts the EnvoyBootstrapHook
// creates Envoy's bootstrap.json configuration based on Connect proxy sidecars
// registered for the task.
func TestTaskRunner_EnvoyBootstrapHook_sidecar_ok(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireConsul(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	alloc := mock.ConnectAlloc()
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
		Kind: structs.NewTaskKind(structs.ConnectProxyPrefix, "foo"),
	}
	tg.Tasks = append(tg.Tasks, sidecarTask)

	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap", alloc.ID)
	defer cleanup()

	// Register Group Services
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testConsul.HTTPAddr
	consulAPIClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	namespacesClient := agentconsul.NewNamespacesClient(consulAPIClient.Namespaces(), consulAPIClient.Agent())

	consulClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), namespacesClient, logger, true)
	go consulClient.Run()
	defer consulClient.Shutdown()
	require.NoError(t, consulClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

	// Run Connect bootstrap Hook
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, consulNamespace, logger))
	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
		TaskEnv: taskenv.NewEmptyTaskEnv(),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	resp := &interfaces.TaskPrestartResponse{}

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), req, resp))

	// Assert it is Done
	require.True(t, resp.Done)

	require.NotNil(t, resp.Env)
	require.Equal(t, "127.0.0.2:19001", resp.Env[envoyAdminBindEnvPrefix+"foo"])

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

func TestTaskRunner_EnvoyBootstrapHook_gateway_ok(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	// Setup an Allocation
	alloc := mock.ConnectIngressGatewayAlloc("bridge")
	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "EnvoyBootstrapIngressGateway", alloc.ID)
	defer cleanupDir()

	// Get a Consul client
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testConsul.HTTPAddr
	consulAPIClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	namespacesClient := agentconsul.NewNamespacesClient(consulAPIClient.Namespaces(), consulAPIClient.Agent())

	// Register Group Services
	serviceClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), namespacesClient, logger, true)
	go serviceClient.Run()
	defer serviceClient.Shutdown()
	require.NoError(t, serviceClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

	// Register Configuration Entry
	ceClient := consulAPIClient.ConfigEntries()
	set, _, err := ceClient.Set(&consulapi.IngressGatewayConfigEntry{
		Kind: consulapi.IngressGateway,
		Name: "gateway-service", // matches job
		Listeners: []consulapi.IngressListener{{
			Port:     2000,
			Protocol: "tcp",
			Services: []consulapi.IngressService{{
				Name: "service1",
			}},
		}},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	// Run Connect bootstrap hook
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, consulNamespace, logger))

	req := &interfaces.TaskPrestartRequest{
		Task:    alloc.Job.TaskGroups[0].Tasks[0],
		TaskDir: allocDir.NewTaskDir(alloc.Job.TaskGroups[0].Tasks[0].Name),
		TaskEnv: taskenv.NewEmptyTaskEnv(),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	var resp interfaces.TaskPrestartResponse

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), req, &resp))

	// Assert the hook is Done
	require.True(t, resp.Done)
	require.NotNil(t, resp.Env)

	// Read the Envoy Config file
	env := map[string]string{
		taskenv.SecretsDir: req.TaskDir.SecretsDir,
	}
	f, err := os.Open(args.ReplaceEnv(structs.EnvoyBootstrapPath, env))
	require.NoError(t, err)
	defer f.Close()

	var out envoyConfig
	require.NoError(t, json.NewDecoder(f).Decode(&out))

	// The only interesting thing on bootstrap is the presence of the cluster,
	// and its associated ID that Nomad sets. Everything is configured at runtime
	// through xDS.
	expID := fmt.Sprintf("_nomad-task-%s-group-web-my-ingress-service-9999", alloc.ID)
	require.Equal(t, expID, out.Node.ID)
	require.Equal(t, "ingress-gateway", out.Node.Cluster)
}

// TestTaskRunner_EnvoyBootstrapHook_Noop asserts that the Envoy bootstrap hook
// is a noop for non-Connect proxy sidecar / gateway tasks.
func TestTaskRunner_EnvoyBootstrapHook_Noop(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	task := alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0]
	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap", alloc.ID)
	defer cleanup()

	// Run Envoy bootstrap Hook. Use invalid Consul address as it should
	// not get hit.
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: "http://127.0.0.2:1",
	}, consulNamespace, logger))
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
	ci.Parallel(t)
	testutil.RequireConsul(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	alloc := mock.ConnectAlloc()
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

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap", alloc.ID)
	defer cleanup()

	// Unlike the successful test above, do NOT register the group services
	// yet. This should cause a recoverable error similar to if Consul was
	// not running.

	// Run Connect bootstrap Hook
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: testConsul.HTTPAddr,
	}, consulNamespace, logger))

	// Lower the allowable wait time for testing
	h.envoyBootstrapWaitTime = 1 * time.Second
	h.envoyBoostrapInitialGap = 100 * time.Millisecond

	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
		TaskEnv: taskenv.NewEmptyTaskEnv(),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	resp := &interfaces.TaskPrestartResponse{}

	// Run the hook
	err := h.Prestart(context.Background(), req, resp)
	require.ErrorIs(t, err, errEnvoyBootstrapError)
	require.True(t, structs.IsRecoverable(err))

	// Assert it is not Done
	require.False(t, resp.Done)

	// Assert no file was written
	_, err = os.Open(filepath.Join(req.TaskDir.SecretsDir, "envoy_bootstrap.json"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func TestTaskRunner_EnvoyBootstrapHook_retryTimeout(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	begin := time.Now()

	// Setup an Allocation
	alloc := mock.ConnectAlloc()
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
		Kind: structs.NewTaskKind(structs.ConnectProxyPrefix, "foo"),
	}
	tg.Tasks = append(tg.Tasks, sidecarTask)
	allocDir, cleanupAlloc := allocdir.TestAllocDir(t, logger, "EnvoyBootstrapRetryTimeout", alloc.ID)
	defer cleanupAlloc()

	// Get a Consul client
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testConsul.HTTPAddr

	// Do NOT register group services, causing the hook to retry until timeout

	// Run Connect bootstrap hook
	h := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, consulNamespace, logger))

	// Keep track of the retry backoff iterations
	iterations := 0

	// Lower the allowable wait time for testing
	h.envoyBootstrapWaitTime = 3 * time.Second
	h.envoyBoostrapInitialGap = 1 * time.Second
	h.envoyBootstrapExpSleep = func(d time.Duration) {
		iterations++
		time.Sleep(d)
	}

	// Create the prestart request
	req := &interfaces.TaskPrestartRequest{
		Task:    sidecarTask,
		TaskDir: allocDir.NewTaskDir(sidecarTask.Name),
		TaskEnv: taskenv.NewEmptyTaskEnv(),
	}
	require.NoError(t, req.TaskDir.Build(false, nil))

	var resp interfaces.TaskPrestartResponse

	// Run the hook and get the error
	err := h.Prestart(context.Background(), req, &resp)
	require.ErrorIs(t, err, errEnvoyBootstrapError)

	// Current time should be at least start time + total wait time
	minimum := begin.Add(h.envoyBootstrapWaitTime)
	require.True(t, time.Now().After(minimum))

	// Should hit at least 2 iterations
	require.Greater(t, 2, iterations)

	// Make sure we captured the recoverable-ness of the error
	_, ok := err.(*structs.RecoverableError)
	require.True(t, ok)

	// Assert the hook is not done (it failed)
	require.False(t, resp.Done)
}

func TestTaskRunner_EnvoyBootstrapHook_extractNameAndKind(t *testing.T) {
	t.Run("connect sidecar", func(t *testing.T) {
		kind, name, err := (*envoyBootstrapHook)(nil).extractNameAndKind(
			structs.NewTaskKind(structs.ConnectProxyPrefix, "foo"),
		)
		require.Nil(t, err)
		require.Equal(t, "connect-proxy", kind)
		require.Equal(t, "foo", name)
	})

	t.Run("connect gateway", func(t *testing.T) {
		kind, name, err := (*envoyBootstrapHook)(nil).extractNameAndKind(
			structs.NewTaskKind(structs.ConnectIngressPrefix, "foo"),
		)
		require.Nil(t, err)
		require.Equal(t, "connect-ingress", kind)
		require.Equal(t, "foo", name)
	})

	t.Run("connect native", func(t *testing.T) {
		_, _, err := (*envoyBootstrapHook)(nil).extractNameAndKind(
			structs.NewTaskKind(structs.ConnectNativePrefix, "foo"),
		)
		require.EqualError(t, err, "envoy must be used as connect sidecar or gateway")
	})

	t.Run("normal task", func(t *testing.T) {
		_, _, err := (*envoyBootstrapHook)(nil).extractNameAndKind(
			structs.TaskKind(""),
		)
		require.EqualError(t, err, "envoy must be used as connect sidecar or gateway")
	})
}

func TestTaskRunner_EnvoyBootstrapHook_grpcAddress(t *testing.T) {
	ci.Parallel(t)

	bridgeH := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(
		mock.ConnectIngressGatewayAlloc("bridge"),
		new(config.ConsulConfig),
		consulNamespace,
		testlog.HCLogger(t),
	))

	hostH := newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(
		mock.ConnectIngressGatewayAlloc("host"),
		new(config.ConsulConfig),
		consulNamespace,
		testlog.HCLogger(t),
	))

	t.Run("environment", func(t *testing.T) {
		env := map[string]string{
			grpcConsulVariable: "1.2.3.4:9000",
		}
		require.Equal(t, "1.2.3.4:9000", bridgeH.grpcAddress(env))
		require.Equal(t, "1.2.3.4:9000", hostH.grpcAddress(env))
	})

	t.Run("defaults", func(t *testing.T) {
		require.Equal(t, "unix://alloc/tmp/consul_grpc.sock", bridgeH.grpcAddress(nil))
		require.Equal(t, "127.0.0.1:8502", hostH.grpcAddress(nil))
	})
}

func TestTaskRunner_EnvoyBootstrapHook_isConnectKind(t *testing.T) {
	ci.Parallel(t)

	require.True(t, isConnectKind(structs.ConnectProxyPrefix))
	require.True(t, isConnectKind(structs.ConnectIngressPrefix))
	require.True(t, isConnectKind(structs.ConnectTerminatingPrefix))
	require.True(t, isConnectKind(structs.ConnectMeshPrefix))
	require.False(t, isConnectKind(""))
	require.False(t, isConnectKind("something"))
}
