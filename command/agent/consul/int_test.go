package consul_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

type mockUpdater struct {
	logger log.Logger
}

func (m *mockUpdater) TaskStateUpdated() {
	m.logger.Named("mock.updater").Debug("Update!")
}

// TestConsul_Integration asserts TaskRunner properly registers and deregisters
// services and checks with Consul using an embedded Consul agent.
func TestConsul_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("-short set; skipping")
	}
	require := require.New(t)

	// Create an embedded Consul server
	testconsul, err := testutil.NewTestServerConfig(func(c *testutil.TestServerConfig) {
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

	conf := config.DefaultConfig()
	conf.Node = mock.Node()
	conf.ConsulConfig.Addr = testconsul.HTTPAddr
	consulConfig, err := conf.ConsulConfig.ApiConfig()
	if err != nil {
		t.Fatalf("error generating consul config: %v", err)
	}

	conf.StateDir, err = ioutil.TempDir("", "nomadtest-consulstate")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(conf.StateDir)
	conf.AllocDir, err = ioutil.TempDir("", "nomdtest-consulalloc")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(conf.AllocDir)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1h",
	}

	// Choose a port that shouldn't be in use
	netResource := &structs.NetworkResource{
		Device:        "eth0",
		IP:            "127.0.0.1",
		MBits:         50,
		ReservedPorts: []structs.Port{{Label: "http", Value: 3}},
	}
	alloc.AllocatedResources.Tasks["web"].Networks[0] = netResource

	task.Services = []*structs.Service{
		{
			Name:      "httpd",
			PortLabel: "http",
			Tags:      []string{"nomad", "test", "http"},
			Checks: []*structs.ServiceCheck{
				{
					Name:     "httpd-http-check",
					Type:     "http",
					Path:     "/",
					Protocol: "http",
					Interval: 9000 * time.Hour,
					Timeout:  1, // fail as fast as possible
				},
				{
					Name:     "httpd-script-check",
					Type:     "script",
					Command:  "/bin/true",
					Interval: 10 * time.Second,
					Timeout:  10 * time.Second,
				},
			},
		},
		{
			Name:      "httpd2",
			PortLabel: "http",
			Tags: []string{
				"test",
				// Use URL-unfriendly tags to test #3620
				"public-test.ettaviation.com:80/ redirect=302,https://test.ettaviation.com",
				"public-test.ettaviation.com:443/",
			},
		},
	}

	logger := testlog.HCLogger(t)
	logUpdate := &mockUpdater{logger}
	allocDir := allocdir.NewAllocDir(logger, filepath.Join(conf.AllocDir, alloc.ID))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("error building alloc dir: %v", err)
	}
	taskDir := allocDir.NewTaskDir(task.Name)
	vclient := vaultclient.NewMockVaultClient()
	consulClient, err := consulapi.NewClient(consulConfig)
	require.Nil(err)

	serviceClient := consul.NewServiceClient(consulClient.Agent(), testlog.HCLogger(t), true)
	defer serviceClient.Shutdown() // just-in-case cleanup
	consulRan := make(chan struct{})
	go func() {
		serviceClient.Run()
		close(consulRan)
	}()

	// Create a closed channel to mock TaskHookCoordinator.startConditionForTask.
	// Closed channel indicates this task is not blocked on prestart hooks.
	closedCh := make(chan struct{})
	close(closedCh)

	// Build the config
	config := &taskrunner.Config{
		Alloc:                alloc,
		ClientConfig:         conf,
		Consul:               serviceClient,
		Task:                 task,
		TaskDir:              taskDir,
		Logger:               logger,
		Vault:                vclient,
		StateDB:              state.NoopDB{},
		StateUpdater:         logUpdate,
		DeviceManager:        devicemanager.NoopMockManager(),
		DriverManager:        drivermanager.TestDriverManager(t),
		StartConditionMetCtx: closedCh,
	}

	tr, err := taskrunner.NewTaskRunner(config)
	require.NoError(err)
	go tr.Run()
	defer func() {
		// Make sure we always shutdown task runner when the test exits
		select {
		case <-tr.WaitCh():
			// Exited cleanly, no need to kill
		default:
			tr.Kill(context.Background(), &structs.TaskEvent{}) // just in case
		}
	}()

	// Block waiting for the service to appear
	catalog := consulClient.Catalog()
	res, meta, err := catalog.Service("httpd2", "test", nil)
	require.Nil(err)

	for i := 0; len(res) == 0 && i < 10; i++ {
		//Expected initial request to fail, do a blocking query
		res, meta, err = catalog.Service("httpd2", "test", &consulapi.QueryOptions{WaitIndex: meta.LastIndex + 1, WaitTime: 3 * time.Second})
		if err != nil {
			t.Fatalf("error querying for service: %v", err)
		}
	}
	require.Len(res, 1)

	// Truncate results
	res = res[:]

	// Assert the service with the checks exists
	for i := 0; len(res) == 0 && i < 10; i++ {
		res, meta, err = catalog.Service("httpd", "http", &consulapi.QueryOptions{WaitIndex: meta.LastIndex + 1, WaitTime: 3 * time.Second})
		require.Nil(err)
	}
	require.Len(res, 1)

	// Assert the script check passes (mock_driver script checks always
	// pass) after having time to run once
	time.Sleep(2 * time.Second)
	checks, _, err := consulClient.Health().Checks("httpd", nil)
	require.Nil(err)
	require.Len(checks, 2)

	for _, check := range checks {
		if expected := "httpd"; check.ServiceName != expected {
			t.Fatalf("expected checks to be for %q but found service name = %q", expected, check.ServiceName)
		}
		switch check.Name {
		case "httpd-http-check":
			// Port check should fail
			if expected := consulapi.HealthCritical; check.Status != expected {
				t.Errorf("expected %q status to be %q but found %q", check.Name, expected, check.Status)
			}
		case "httpd-script-check":
			// mock_driver script checks always succeed
			if expected := consulapi.HealthPassing; check.Status != expected {
				t.Errorf("expected %q status to be %q but found %q", check.Name, expected, check.Status)
			}
		default:
			t.Errorf("unexpected check %q with status %q", check.Name, check.Status)
		}
	}

	// Assert the service client returns all the checks for the allocation.
	reg, err := serviceClient.AllocRegistrations(alloc.ID)
	if err != nil {
		t.Fatalf("unexpected error retrieving allocation checks: %v", err)
	}
	if reg == nil {
		t.Fatalf("Unexpected nil allocation registration")
	}
	if snum := reg.NumServices(); snum != 2 {
		t.Fatalf("Unexpected number of services registered. Got %d; want 2", snum)
	}
	if cnum := reg.NumChecks(); cnum != 2 {
		t.Fatalf("Unexpected number of checks registered. Got %d; want 2", cnum)
	}

	logger.Debug("killing task")

	// Kill the task
	tr.Kill(context.Background(), &structs.TaskEvent{})

	select {
	case <-tr.WaitCh():
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for Run() to exit")
	}

	// Shutdown Consul ServiceClient to ensure all pending operations complete
	if err := serviceClient.Shutdown(); err != nil {
		t.Errorf("error shutting down Consul ServiceClient: %v", err)
	}

	// Ensure Consul is clean
	services, _, err := catalog.Services(nil)
	require.Nil(err)
	require.Len(services, 1)
	require.Contains(services, "consul")
}
