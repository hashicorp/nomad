package consul_test

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testLogger() *log.Logger {
	if testing.Verbose() {
		return log.New(os.Stderr, "", log.LstdFlags)
	}
	return log.New(ioutil.Discard, "", 0)
}

// TestConsul_Integration asserts TaskRunner properly registers and deregisters
// services and checks with Consul using an embedded Consul agent.
func TestConsul_Integration(t *testing.T) {
	if _, ok := driver.BuiltinDrivers["mock_driver"]; !ok {
		t.Skip(`test requires mock_driver; run with "-tags nomad_test"`)
	}
	if testing.Short() {
		t.Skip("-short set; skipping")
	}
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

	tmp, err := ioutil.TempFile("", "state-db")
	if err != nil {
		t.Fatalf("error creating state db file: %v", err)
	}
	db, err := bolt.Open(tmp.Name(), 0600, nil)
	if err != nil {
		t.Fatalf("error creating state db: %v", err)
	}

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1h",
	}
	// Choose a port that shouldn't be in use
	task.Resources.Networks[0].ReservedPorts = []structs.Port{{Label: "http", Value: 3}}
	task.Services = []*structs.Service{
		{
			Name:      "httpd",
			PortLabel: "http",
			Tags:      []string{"nomad", "test", "http"},
			Checks: []*structs.ServiceCheck{
				{
					Name:      "httpd-http-check",
					Type:      "http",
					Path:      "/",
					Protocol:  "http",
					PortLabel: "http",
					Interval:  9000 * time.Hour,
					Timeout:   1, // fail as fast as possible
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
			Tags:      []string{"test", "http2"},
		},
	}

	logger := testLogger()
	logUpdate := func(name, state string, event *structs.TaskEvent) {
		logger.Printf("[TEST] test.updater: name=%q state=%q event=%v", name, state, event)
	}
	allocDir := allocdir.NewAllocDir(logger, filepath.Join(conf.AllocDir, alloc.ID))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("error building alloc dir: %v", err)
	}
	taskDir := allocDir.NewTaskDir(task.Name)
	vclient := vaultclient.NewMockVaultClient()
	consulClient, err := consulapi.NewClient(consulConfig)
	if err != nil {
		t.Fatalf("error creating consul client: %v", err)
	}
	serviceClient := consul.NewServiceClient(consulClient.Agent(), true, logger)
	defer serviceClient.Shutdown() // just-in-case cleanup
	consulRan := make(chan struct{})
	go func() {
		serviceClient.Run()
		close(consulRan)
	}()
	tr := client.NewTaskRunner(logger, conf, db, logUpdate, taskDir, alloc, task, vclient, serviceClient)
	tr.MarkReceived()
	go tr.Run()
	defer func() {
		// Make sure we always shutdown task runner when the test exits
		select {
		case <-tr.WaitCh():
			// Exited cleanly, no need to kill
		default:
			tr.Kill("", "", true) // just in case
		}
	}()

	// Block waiting for the service to appear
	catalog := consulClient.Catalog()
	res, meta, err := catalog.Service("httpd2", "test", nil)
	for i := 0; len(res) == 0 && i < 10; i++ {
		//Expected initial request to fail, do a blocking query
		res, meta, err = catalog.Service("httpd2", "test", &consulapi.QueryOptions{WaitIndex: meta.LastIndex + 1, WaitTime: 3 * time.Second})
		if err != nil {
			t.Fatalf("error querying for service: %v", err)
		}
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", len(res), res)
	}
	res = res[:]

	// Assert the service with the checks exists
	for i := 0; len(res) == 0 && i < 10; i++ {
		res, meta, err = catalog.Service("httpd", "http", &consulapi.QueryOptions{WaitIndex: meta.LastIndex + 1, WaitTime: 3 * time.Second})
		if err != nil {
			t.Fatalf("error querying for service: %v", err)
		}
	}
	if len(res) != 1 {
		t.Fatalf("exepcted 1 service but found %d:\n%#v", len(res), res)
	}

	// Assert the script check passes (mock_driver script checks always
	// pass) after having time to run once
	time.Sleep(2 * time.Second)
	checks, _, err := consulClient.Health().Checks("httpd", nil)
	if err != nil {
		t.Fatalf("error querying checks: %v", err)
	}
	if expected := 2; len(checks) != expected {
		t.Fatalf("expected %d checks but found %d:\n%#v", expected, len(checks), checks)
	}
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
	checksOut, err := serviceClient.Checks(alloc)
	if err != nil {
		t.Fatalf("unexpected error retrieving allocation checks: %v", err)
	}

	if l := len(checksOut); l != 2 {
		t.Fatalf("got %d checks; want %d", l, 2)
	}

	logger.Printf("[TEST] consul.test: killing task")

	// Kill the task
	tr.Kill("", "", false)

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
	if err != nil {
		t.Fatalf("error query services: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected only 1 service in Consul but found %d:\n%#v", len(services), services)
	}
	if _, ok := services["consul"]; !ok {
		t.Fatalf(`expected only the "consul" key in Consul but found: %#v`, services)
	}
}
