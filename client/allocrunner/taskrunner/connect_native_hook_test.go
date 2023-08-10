// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	consultest "github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func getTestConsul(t *testing.T) *consultest.TestServer {
	testConsul, err := consultest.NewTestServerConfigT(t, func(c *consultest.TestServerConfig) {
		c.Peering = nil         // fix for older versions of Consul (<1.13.0) that don't support peering
		if !testing.Verbose() { // disable consul logging if -v not set
			c.Stdout = io.Discard
			c.Stderr = io.Discard
		}
	})
	require.NoError(t, err, "failed to start test consul server")
	return testConsul
}

func TestConnectNativeHook_Name(t *testing.T) {
	ci.Parallel(t)
	name := new(connectNativeHook).Name()
	require.Equal(t, "connect_native", name)
}

func setupCertDirs(t *testing.T) (string, string) {
	fd, err := os.CreateTemp(t.TempDir(), "connect_native_testcert")
	require.NoError(t, err)
	_, err = fd.WriteString("ABCDEF")
	require.NoError(t, err)
	err = fd.Close()
	require.NoError(t, err)

	return fd.Name(), t.TempDir()
}

func TestConnectNativeHook_copyCertificate(t *testing.T) {
	ci.Parallel(t)

	f, d := setupCertDirs(t)

	t.Run("no source", func(t *testing.T) {
		err := new(connectNativeHook).copyCertificate("", d, "out.pem")
		require.NoError(t, err)
	})

	t.Run("normal", func(t *testing.T) {
		err := new(connectNativeHook).copyCertificate(f, d, "out.pem")
		require.NoError(t, err)
		b, err := os.ReadFile(filepath.Join(d, "out.pem"))
		require.NoError(t, err)
		require.Equal(t, "ABCDEF", string(b))
	})
}

func TestConnectNativeHook_copyCertificates(t *testing.T) {
	ci.Parallel(t)

	f, d := setupCertDirs(t)

	t.Run("normal", func(t *testing.T) {
		err := new(connectNativeHook).copyCertificates(consulTransportConfig{
			CAFile:   f,
			CertFile: f,
			KeyFile:  f,
		}, d)
		require.NoError(t, err)
		ls, err := os.ReadDir(d)
		require.NoError(t, err)
		require.Equal(t, 3, len(ls))
	})

	t.Run("no source", func(t *testing.T) {
		err := new(connectNativeHook).copyCertificates(consulTransportConfig{
			CAFile:   "/does/not/exist.pem",
			CertFile: "/does/not/exist.pem",
			KeyFile:  "/does/not/exist.pem",
		}, d)
		require.EqualError(t, err, "failed to open consul TLS certificate: open /does/not/exist.pem: no such file or directory")
	})
}

func TestConnectNativeHook_tlsEnv(t *testing.T) {
	ci.Parallel(t)

	// the hook config comes from client config
	emptyHook := new(connectNativeHook)
	fullHook := &connectNativeHook{
		consulConfig: consulTransportConfig{
			Auth:      "user:password",
			SSL:       "true",
			VerifySSL: "true",
			CAFile:    "/not/real/ca.pem",
			CertFile:  "/not/real/cert.pem",
			KeyFile:   "/not/real/key.pem",
		},
	}

	// existing config from task env block
	taskEnv := map[string]string{
		"CONSUL_CACERT":          "fakeCA.pem",
		"CONSUL_CLIENT_CERT":     "fakeCert.pem",
		"CONSUL_CLIENT_KEY":      "fakeKey.pem",
		"CONSUL_HTTP_AUTH":       "foo:bar",
		"CONSUL_HTTP_SSL":        "false",
		"CONSUL_HTTP_SSL_VERIFY": "false",
	}

	t.Run("empty hook and empty task", func(t *testing.T) {
		result := emptyHook.tlsEnv(nil)
		require.Empty(t, result)
	})

	t.Run("empty hook and non-empty task", func(t *testing.T) {
		result := emptyHook.tlsEnv(taskEnv)
		require.Empty(t, result) // tlsEnv only overrides; task env is actually set elsewhere
	})

	t.Run("non-empty hook and empty task", func(t *testing.T) {
		result := fullHook.tlsEnv(nil)
		require.Equal(t, map[string]string{
			// ca files are specifically copied into FS namespace
			"CONSUL_CACERT":          "/secrets/consul_ca_file.pem",
			"CONSUL_CLIENT_CERT":     "/secrets/consul_cert_file.pem",
			"CONSUL_CLIENT_KEY":      "/secrets/consul_key_file.pem",
			"CONSUL_HTTP_SSL":        "true",
			"CONSUL_HTTP_SSL_VERIFY": "true",
		}, result)
	})

	t.Run("non-empty hook and non-empty task", func(t *testing.T) {
		result := fullHook.tlsEnv(taskEnv) // task env takes precedence, nothing gets set here
		require.Empty(t, result)
	})
}

func TestConnectNativeHook_bridgeEnv_bridge(t *testing.T) {
	ci.Parallel(t)

	t.Run("without tls", func(t *testing.T) {
		hook := new(connectNativeHook)
		hook.alloc = mock.ConnectNativeAlloc("bridge")

		t.Run("consul address env not preconfigured", func(t *testing.T) {
			result := hook.bridgeEnv(nil)
			require.Equal(t, map[string]string{
				"CONSUL_HTTP_ADDR": "unix:///alloc/tmp/consul_http.sock",
			}, result)
		})

		t.Run("consul address env is preconfigured", func(t *testing.T) {
			result := hook.bridgeEnv(map[string]string{
				"CONSUL_HTTP_ADDR": "10.1.1.1",
			})
			require.Empty(t, result)
		})
	})

	t.Run("with tls", func(t *testing.T) {
		hook := new(connectNativeHook)
		hook.alloc = mock.ConnectNativeAlloc("bridge")
		hook.consulConfig.SSL = "true"

		t.Run("consul tls server name not preconfigured", func(t *testing.T) {
			result := hook.bridgeEnv(nil)
			require.Equal(t, map[string]string{
				"CONSUL_HTTP_ADDR":       "unix:///alloc/tmp/consul_http.sock",
				"CONSUL_TLS_SERVER_NAME": "localhost",
			}, result)
		})

		t.Run("consul tls server name preconfigured", func(t *testing.T) {
			result := hook.bridgeEnv(map[string]string{
				"CONSUL_HTTP_ADDR":       "10.1.1.1",
				"CONSUL_TLS_SERVER_NAME": "consul.local",
			})
			require.Empty(t, result)
		})
	})
}

func TestConnectNativeHook_bridgeEnv_host(t *testing.T) {
	ci.Parallel(t)

	hook := new(connectNativeHook)
	hook.alloc = mock.ConnectNativeAlloc("host")

	t.Run("consul address env not preconfigured", func(t *testing.T) {
		result := hook.bridgeEnv(nil)
		require.Empty(t, result)
	})

	t.Run("consul address env is preconfigured", func(t *testing.T) {
		result := hook.bridgeEnv(map[string]string{
			"CONSUL_HTTP_ADDR": "10.1.1.1",
		})
		require.Empty(t, result)
	})
}

func TestConnectNativeHook_hostEnv_host(t *testing.T) {
	ci.Parallel(t)

	hook := new(connectNativeHook)
	hook.alloc = mock.ConnectNativeAlloc("host")
	hook.consulConfig.HTTPAddr = "http://1.2.3.4:9999"

	t.Run("consul address env not preconfigured", func(t *testing.T) {
		result := hook.hostEnv(nil)
		require.Equal(t, map[string]string{
			"CONSUL_HTTP_ADDR": "http://1.2.3.4:9999",
		}, result)
	})

	t.Run("consul address env is preconfigured", func(t *testing.T) {
		result := hook.hostEnv(map[string]string{
			"CONSUL_HTTP_ADDR": "10.1.1.1",
		})
		require.Empty(t, result)
	})
}

func TestConnectNativeHook_hostEnv_bridge(t *testing.T) {
	ci.Parallel(t)

	hook := new(connectNativeHook)
	hook.alloc = mock.ConnectNativeAlloc("bridge")
	hook.consulConfig.HTTPAddr = "http://1.2.3.4:9999"

	t.Run("consul address env not preconfigured", func(t *testing.T) {
		result := hook.hostEnv(nil)
		require.Empty(t, result)
	})

	t.Run("consul address env is preconfigured", func(t *testing.T) {
		result := hook.hostEnv(map[string]string{
			"CONSUL_HTTP_ADDR": "10.1.1.1",
		})
		require.Empty(t, result)
	})
}

func TestTaskRunner_ConnectNativeHook_Noop(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	task := alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0]
	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "ConnectNative", alloc.ID)
	defer cleanup()

	// run the connect native hook. use invalid consul address as it should not get hit
	h := newConnectNativeHook(newConnectNativeHookConfig(alloc, &config.ConsulConfig{
		Addr: "http://127.0.0.2:1",
	}, logger))

	request := &interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: allocDir.NewTaskDir(task.Name),
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	response := new(interfaces.TaskPrestartResponse)

	// Run the hook
	require.NoError(t, h.Prestart(context.Background(), request, response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert no environment variables configured to be set
	require.Empty(t, response.Env)

	// Assert secrets dir is empty (no TLS config set)
	checkFilesInDir(t, request.TaskDir.SecretsDir,
		nil,
		[]string{sidsTokenFile, secretCAFilename, secretCertfileFilename, secretKeyfileFilename},
	)
}

func TestTaskRunner_ConnectNativeHook_Ok(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireConsul(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{{Mode: "host", IP: "1.1.1.1"}}
	tg := alloc.Job.TaskGroups[0]
	tg.Services = []*structs.Service{{
		Name:     "cn-service",
		TaskName: tg.Tasks[0].Name,
		Connect: &structs.ConsulConnect{
			Native: true,
		}},
	}
	tg.Tasks[0].Kind = structs.NewTaskKind("connect-native", "cn-service")

	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "ConnectNative", alloc.ID)
	defer cleanup()

	// register group services
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testConsul.HTTPAddr
	consulAPIClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	namespacesClient := agentconsul.NewNamespacesClient(consulAPIClient.Namespaces(), consulAPIClient.Agent())

	consulClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), namespacesClient, logger, true)
	go consulClient.Run()
	defer consulClient.Shutdown()
	require.NoError(t, consulClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

	// Run Connect Native hook
	h := newConnectNativeHook(newConnectNativeHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, logger))
	request := &interfaces.TaskPrestartRequest{
		Task:    tg.Tasks[0],
		TaskDir: allocDir.NewTaskDir(tg.Tasks[0].Name),
		TaskEnv: taskenv.NewEmptyTaskEnv(),
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	response := new(interfaces.TaskPrestartResponse)

	// Run the Connect Native hook
	require.NoError(t, h.Prestart(context.Background(), request, response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert only CONSUL_HTTP_ADDR env variable is set
	require.Equal(t, map[string]string{"CONSUL_HTTP_ADDR": testConsul.HTTPAddr}, response.Env)

	// Assert no secrets were written
	checkFilesInDir(t, request.TaskDir.SecretsDir,
		nil,
		[]string{sidsTokenFile, secretCAFilename, secretCertfileFilename, secretKeyfileFilename},
	)
}

func TestTaskRunner_ConnectNativeHook_with_SI_token(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireConsul(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{{Mode: "host", IP: "1.1.1.1"}}
	tg := alloc.Job.TaskGroups[0]
	tg.Services = []*structs.Service{{
		Name:     "cn-service",
		TaskName: tg.Tasks[0].Name,
		Connect: &structs.ConsulConnect{
			Native: true,
		}},
	}
	tg.Tasks[0].Kind = structs.NewTaskKind("connect-native", "cn-service")

	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "ConnectNative", alloc.ID)
	defer cleanup()

	// register group services
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testConsul.HTTPAddr
	consulAPIClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	namespacesClient := agentconsul.NewNamespacesClient(consulAPIClient.Namespaces(), consulAPIClient.Agent())

	consulClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), namespacesClient, logger, true)
	go consulClient.Run()
	defer consulClient.Shutdown()
	require.NoError(t, consulClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

	// Run Connect Native hook
	h := newConnectNativeHook(newConnectNativeHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,
	}, logger))
	request := &interfaces.TaskPrestartRequest{
		Task:    tg.Tasks[0],
		TaskDir: allocDir.NewTaskDir(tg.Tasks[0].Name),
		TaskEnv: taskenv.NewEmptyTaskEnv(),
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	// Insert service identity token in the secrets directory
	token := uuid.Generate()
	siTokenFile := filepath.Join(request.TaskDir.SecretsDir, sidsTokenFile)
	err = os.WriteFile(siTokenFile, []byte(token), 0440)
	require.NoError(t, err)

	response := new(interfaces.TaskPrestartResponse)
	response.Env = make(map[string]string)

	// Run the Connect Native hook
	require.NoError(t, h.Prestart(context.Background(), request, response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert environment variable for token is set
	require.NotEmpty(t, response.Env)
	require.Equal(t, token, response.Env["CONSUL_HTTP_TOKEN"])

	// Assert no additional secrets were written
	checkFilesInDir(t, request.TaskDir.SecretsDir,
		[]string{sidsTokenFile},
		[]string{secretCAFilename, secretCertfileFilename, secretKeyfileFilename},
	)
}

func TestTaskRunner_ConnectNativeHook_shareTLS(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireConsul(t)

	try := func(t *testing.T, shareSSL *bool) {
		fakeCert, _ := setupCertDirs(t)

		testConsul := getTestConsul(t)
		defer testConsul.Stop()

		alloc := mock.Alloc()
		alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{{Mode: "host", IP: "1.1.1.1"}}
		tg := alloc.Job.TaskGroups[0]
		tg.Services = []*structs.Service{{
			Name:     "cn-service",
			TaskName: tg.Tasks[0].Name,
			Connect: &structs.ConsulConnect{
				Native: true,
			}},
		}
		tg.Tasks[0].Kind = structs.NewTaskKind("connect-native", "cn-service")

		logger := testlog.HCLogger(t)

		allocDir, cleanup := allocdir.TestAllocDir(t, logger, "ConnectNative", alloc.ID)
		defer cleanup()

		// register group services
		consulConfig := consulapi.DefaultConfig()
		consulConfig.Address = testConsul.HTTPAddr
		consulAPIClient, err := consulapi.NewClient(consulConfig)
		require.NoError(t, err)
		namespacesClient := agentconsul.NewNamespacesClient(consulAPIClient.Namespaces(), consulAPIClient.Agent())

		consulClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), namespacesClient, logger, true)
		go consulClient.Run()
		defer consulClient.Shutdown()
		require.NoError(t, consulClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

		// Run Connect Native hook
		h := newConnectNativeHook(newConnectNativeHookConfig(alloc, &config.ConsulConfig{
			Addr: consulConfig.Address,

			// TLS config consumed by native application
			ShareSSL:  shareSSL,
			EnableSSL: pointer.Of(true),
			VerifySSL: pointer.Of(true),
			CAFile:    fakeCert,
			CertFile:  fakeCert,
			KeyFile:   fakeCert,
			Auth:      "user:password",
			Token:     uuid.Generate(),
		}, logger))
		request := &interfaces.TaskPrestartRequest{
			Task:    tg.Tasks[0],
			TaskDir: allocDir.NewTaskDir(tg.Tasks[0].Name),
			TaskEnv: taskenv.NewEmptyTaskEnv(), // nothing set in env block
		}
		require.NoError(t, request.TaskDir.Build(false, nil))

		response := new(interfaces.TaskPrestartResponse)
		response.Env = make(map[string]string)

		// Run the Connect Native hook
		require.NoError(t, h.Prestart(context.Background(), request, response))

		// Assert the hook is Done
		require.True(t, response.Done)

		// Remove variables we are not interested in
		delete(response.Env, "CONSUL_HTTP_ADDR")

		// Assert environment variable for token is set
		require.NotEmpty(t, response.Env)
		require.Equal(t, map[string]string{
			"CONSUL_CACERT":          "/secrets/consul_ca_file.pem",
			"CONSUL_CLIENT_CERT":     "/secrets/consul_cert_file.pem",
			"CONSUL_CLIENT_KEY":      "/secrets/consul_key_file.pem",
			"CONSUL_HTTP_SSL":        "true",
			"CONSUL_HTTP_SSL_VERIFY": "true",
		}, response.Env)
		require.NotContains(t, response.Env, "CONSUL_HTTP_AUTH")  // explicitly not shared
		require.NotContains(t, response.Env, "CONSUL_HTTP_TOKEN") // explicitly not shared

		// Assert 3 pem files were written
		checkFilesInDir(t, request.TaskDir.SecretsDir,
			[]string{secretCAFilename, secretCertfileFilename, secretKeyfileFilename},
			[]string{sidsTokenFile},
		)
	}

	// The default behavior is that share_ssl is true (similar to allow_unauthenticated)
	// so make sure an unset value turns the feature on.

	t.Run("share_ssl is true", func(t *testing.T) {
		try(t, pointer.Of(true))
	})

	t.Run("share_ssl is nil", func(t *testing.T) {
		try(t, nil)
	})
}

func checkFilesInDir(t *testing.T, dir string, includes, excludes []string) {
	ls, err := os.ReadDir(dir)
	require.NoError(t, err)

	var present []string
	for _, fInfo := range ls {
		present = append(present, fInfo.Name())
	}

	for _, filename := range includes {
		require.Contains(t, present, filename)
	}
	for _, filename := range excludes {
		require.NotContains(t, present, filename)
	}
}

func TestTaskRunner_ConnectNativeHook_shareTLS_override(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireConsul(t)

	fakeCert, _ := setupCertDirs(t)

	testConsul := getTestConsul(t)
	defer testConsul.Stop()

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{{Mode: "host", IP: "1.1.1.1"}}
	tg := alloc.Job.TaskGroups[0]
	tg.Services = []*structs.Service{{
		Name:     "cn-service",
		TaskName: tg.Tasks[0].Name,
		Connect: &structs.ConsulConnect{
			Native: true,
		}},
	}
	tg.Tasks[0].Kind = structs.NewTaskKind("connect-native", "cn-service")

	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "ConnectNative", alloc.ID)
	defer cleanup()

	// register group services
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testConsul.HTTPAddr
	consulAPIClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	namespacesClient := agentconsul.NewNamespacesClient(consulAPIClient.Namespaces(), consulAPIClient.Agent())

	consulClient := agentconsul.NewServiceClient(consulAPIClient.Agent(), namespacesClient, logger, true)
	go consulClient.Run()
	defer consulClient.Shutdown()
	require.NoError(t, consulClient.RegisterWorkload(agentconsul.BuildAllocServices(mock.Node(), alloc, agentconsul.NoopRestarter())))

	// Run Connect Native hook
	h := newConnectNativeHook(newConnectNativeHookConfig(alloc, &config.ConsulConfig{
		Addr: consulConfig.Address,

		// TLS config consumed by native application
		ShareSSL:  pointer.Of(true),
		EnableSSL: pointer.Of(true),
		VerifySSL: pointer.Of(true),
		CAFile:    fakeCert,
		CertFile:  fakeCert,
		KeyFile:   fakeCert,
		Auth:      "user:password",
	}, logger))

	taskEnv := taskenv.NewEmptyTaskEnv()
	taskEnv.EnvMap = map[string]string{
		"CONSUL_CACERT":          "/foo/ca.pem",
		"CONSUL_CLIENT_CERT":     "/foo/cert.pem",
		"CONSUL_CLIENT_KEY":      "/foo/key.pem",
		"CONSUL_HTTP_AUTH":       "foo:bar",
		"CONSUL_HTTP_SSL_VERIFY": "false",
		"CONSUL_HTTP_ADDR":       "localhost:8500",
		// CONSUL_HTTP_SSL (check the default value is assumed from client config)
	}

	request := &interfaces.TaskPrestartRequest{
		Task:    tg.Tasks[0],
		TaskDir: allocDir.NewTaskDir(tg.Tasks[0].Name),
		TaskEnv: taskEnv, // env block is configured w/ non-default tls configs
	}
	require.NoError(t, request.TaskDir.Build(false, nil))

	response := new(interfaces.TaskPrestartResponse)
	response.Env = make(map[string]string)

	// Run the Connect Native hook
	require.NoError(t, h.Prestart(context.Background(), request, response))

	// Assert the hook is Done
	require.True(t, response.Done)

	// Assert environment variable for CONSUL_HTTP_SSL is set, because it was
	// the only one not overridden by task env block config
	require.NotEmpty(t, response.Env)
	require.Equal(t, map[string]string{
		"CONSUL_HTTP_SSL": "true",
	}, response.Env)

	// Assert 3 pem files were written (even though they will be ignored)
	// as this is gated by share_tls, not the presense of ca environment variables.
	checkFilesInDir(t, request.TaskDir.SecretsDir,
		[]string{secretCAFilename, secretCertfileFilename, secretKeyfileFilename},
		[]string{sidsTokenFile},
	)
}
