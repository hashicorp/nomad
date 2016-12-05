package consul

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

const (
	allocID          = "12"
	serviceRegPrefix = "test"
	serviceGroupName = "executor"
)

var logger = log.New(os.Stdout, "", log.LstdFlags)

func TestSyncNow(t *testing.T) {
	cs, testconsul := testConsul(t)
	defer cs.Shutdown()
	defer testconsul.Stop()

	cs.SetAddrFinder(func(h string) (string, int) {
		a, pstr, _ := net.SplitHostPort(h)
		p, _ := net.LookupPort("tcp", pstr)
		return a, p
	})
	cs.syncInterval = 9000 * time.Hour

	service := &structs.Service{Name: "foo1", Tags: []string{"a", "b"}}
	services := map[ServiceKey]*structs.Service{
		GenerateServiceKey(service): service,
	}

	// Run syncs once on startup and then blocks forever
	go cs.Run()

	if err := cs.SetServices(serviceGroupName, services); err != nil {
		t.Fatalf("error setting services: %v", err)
	}

	synced := false
	for i := 0; !synced && i < 10; i++ {
		time.Sleep(250 * time.Millisecond)
		agentServices, err := cs.queryAgentServices()
		if err != nil {
			t.Fatalf("error querying consul services: %v", err)
		}
		synced = len(agentServices) == 1
	}
	if !synced {
		t.Fatalf("initial sync never occurred")
	}

	// SetServices again should cause another sync
	service1 := &structs.Service{Name: "foo1", Tags: []string{"Y", "Z"}}
	service2 := &structs.Service{Name: "bar"}
	services = map[ServiceKey]*structs.Service{
		GenerateServiceKey(service1): service1,
		GenerateServiceKey(service2): service2,
	}

	if err := cs.SetServices(serviceGroupName, services); err != nil {
		t.Fatalf("error setting services: %v", err)
	}

	synced = false
	for i := 0; !synced && i < 10; i++ {
		time.Sleep(250 * time.Millisecond)
		agentServices, err := cs.queryAgentServices()
		if err != nil {
			t.Fatalf("error querying consul services: %v", err)
		}
		synced = len(agentServices) == 2
	}
	if !synced {
		t.Fatalf("SetServices didn't sync immediately")
	}
}

func TestCheckRegistration(t *testing.T) {
	cs, err := NewSyncer(config.DefaultConsulConfig(), make(chan struct{}), logger)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	check1 := structs.ServiceCheck{
		Name:          "check-foo-1",
		Type:          structs.ServiceCheckTCP,
		Interval:      30 * time.Second,
		Timeout:       5 * time.Second,
		InitialStatus: api.HealthPassing,
	}
	check2 := structs.ServiceCheck{
		Name:      "check1",
		Type:      "tcp",
		PortLabel: "port2",
		Interval:  3 * time.Second,
		Timeout:   1 * time.Second,
	}
	check3 := structs.ServiceCheck{
		Name:      "check3",
		Type:      "http",
		PortLabel: "port3",
		Path:      "/health?p1=1&p2=2",
		Interval:  3 * time.Second,
		Timeout:   1 * time.Second,
	}
	service1 := structs.Service{
		Name:      "foo-1",
		Tags:      []string{"tag1", "tag2"},
		PortLabel: "port1",
		Checks: []*structs.ServiceCheck{
			&check1, &check2,
		},
	}
	task := structs.Task{
		Name:     "foo",
		Services: []*structs.Service{&service1},
		Resources: &structs.Resources{
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					IP: "10.10.11.5",
					DynamicPorts: []structs.Port{
						structs.Port{
							Label: "port1",
							Value: 20002,
						},
						structs.Port{
							Label: "port2",
							Value: 20003,
						},
						structs.Port{
							Label: "port3",
							Value: 20004,
						},
					},
				},
			},
		},
	}
	cs.SetAddrFinder(task.FindHostAndPortFor)
	srvReg, _ := cs.createService(&service1, "domain", "key")
	check1Reg, _ := cs.createCheckReg(&check1, srvReg)
	check2Reg, _ := cs.createCheckReg(&check2, srvReg)
	check3Reg, _ := cs.createCheckReg(&check3, srvReg)

	expected := "10.10.11.5:20002"
	if check1Reg.TCP != expected {
		t.Fatalf("expected: %v, actual: %v", expected, check1Reg.TCP)
	}

	expected = "10.10.11.5:20003"
	if check2Reg.TCP != expected {
		t.Fatalf("expected: %v, actual: %v", expected, check2Reg.TCP)
	}

	expected = "http://10.10.11.5:20004/health?p1=1&p2=2"
	if check3Reg.HTTP != expected {
		t.Fatalf("expected: %v, actual: %v", expected, check3Reg.HTTP)
	}

	expected = api.HealthPassing
	if check1Reg.Status != expected {
		t.Fatalf("expected: %v, actual: %v", expected, check1Reg.Status)
	}
}

// testConsul returns a Syncer configured with an embedded Consul server.
//
// Callers must defer Syncer.Shutdown() and TestServer.Stop()
//
func testConsul(t *testing.T) (*Syncer, *testutil.TestServer) {
	// Create an embedded Consul server
	testconsul := testutil.NewTestServerConfig(t, func(c *testutil.TestServerConfig) {
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		}
	})

	// Configure Syncer to talk to the test server
	cconf := config.DefaultConsulConfig()
	cconf.Addr = testconsul.HTTPAddr

	cs, err := NewSyncer(cconf, nil, logger)
	if err != nil {
		t.Fatalf("Error creating Syncer: %v", err)
	}
	return cs, testconsul
}

func TestConsulServiceRegisterServices(t *testing.T) {
	cs, testconsul := testConsul(t)
	defer cs.Shutdown()
	defer testconsul.Stop()

	service1 := &structs.Service{Name: "foo", Tags: []string{"a", "b"}}
	service2 := &structs.Service{Name: "foo"}
	services := map[ServiceKey]*structs.Service{
		GenerateServiceKey(service1): service1,
		GenerateServiceKey(service2): service2,
	}

	// Call SetServices to update services in consul
	if err := cs.SetServices(serviceGroupName, services); err != nil {
		t.Fatalf("error setting services: %v", err)
	}

	// Manually call SyncServers to cause a synchronous consul update
	if err := cs.SyncServices(); err != nil {
		t.Fatalf("error syncing services: %v", err)
	}

	numservices := len(cs.flattenedServices())
	if numservices != 2 {
		t.Fatalf("expected 2 services but found %d", numservices)
	}

	numchecks := len(cs.flattenedChecks())
	if numchecks != 0 {
		t.Fatalf("expected 0 checks but found %d", numchecks)
	}

	// Assert services are in consul
	agentServices, err := cs.client.Agent().Services()
	if err != nil {
		t.Fatalf("error querying consul services: %v", err)
	}
	found := 0
	for id, as := range agentServices {
		if id == "consul" {
			found++
			continue
		}
		if _, ok := services[ServiceKey(as.Service)]; ok {
			found++
			continue
		}
		t.Errorf("unexpected service in consul: %s", id)
	}
	if found != 3 {
		t.Fatalf("expected 3 services in consul but found %d:\nconsul: %#v", len(agentServices), agentServices)
	}

	agentChecks, err := cs.queryChecks()
	if err != nil {
		t.Fatalf("error querying consul checks: %v", err)
	}
	if len(agentChecks) != numchecks {
		t.Fatalf("expected %d checks in consul but found %d:\n%#v", numservices, len(agentChecks), agentChecks)
	}
}

func TestConsulServiceUpdateService(t *testing.T) {
	cs, testconsul := testConsul(t)
	defer cs.Shutdown()
	defer testconsul.Stop()

	cs.SetAddrFinder(func(h string) (string, int) {
		a, pstr, _ := net.SplitHostPort(h)
		p, _ := net.LookupPort("tcp", pstr)
		return a, p
	})

	service1 := &structs.Service{Name: "foo1", Tags: []string{"a", "b"}}
	service2 := &structs.Service{Name: "foo2"}
	services := map[ServiceKey]*structs.Service{
		GenerateServiceKey(service1): service1,
		GenerateServiceKey(service2): service2,
	}
	if err := cs.SetServices(serviceGroupName, services); err != nil {
		t.Fatalf("error setting services: %v", err)
	}
	if err := cs.SyncServices(); err != nil {
		t.Fatalf("error syncing services: %v", err)
	}

	// Now update both services
	service1 = &structs.Service{Name: "foo1", Tags: []string{"a", "z"}}
	service2 = &structs.Service{Name: "foo2", PortLabel: ":8899"}
	service3 := &structs.Service{Name: "foo3"}
	services = map[ServiceKey]*structs.Service{
		GenerateServiceKey(service1): service1,
		GenerateServiceKey(service2): service2,
		GenerateServiceKey(service3): service3,
	}
	if err := cs.SetServices(serviceGroupName, services); err != nil {
		t.Fatalf("error setting services: %v", err)
	}
	if err := cs.SyncServices(); err != nil {
		t.Fatalf("error syncing services: %v", err)
	}

	agentServices, err := cs.queryAgentServices()
	if err != nil {
		t.Fatalf("error querying consul services: %v", err)
	}
	if len(agentServices) != 3 {
		t.Fatalf("expected 3 services in consul but found %d:\n%#v", len(agentServices), agentServices)
	}
	consulServices := make(map[string]*api.AgentService, 3)
	for _, as := range agentServices {
		consulServices[as.ID] = as
	}

	found := 0
	for _, s := range cs.flattenedServices() {
		// Assert sure changes were applied to internal state
		switch s.Name {
		case "foo1":
			found++
			if !reflect.DeepEqual(service1.Tags, s.Tags) {
				t.Errorf("incorrect tags on foo1:\n  expected: %v\n  found: %v", service1.Tags, s.Tags)
			}
		case "foo2":
			found++
			if s.Address != "" {
				t.Errorf("expected empty host on foo2 but found %q", s.Address)
			}
			if s.Port != 8899 {
				t.Errorf("expected port 8899 on foo2 but found %d", s.Port)
			}
		case "foo3":
			found++
		default:
			t.Errorf("unexpected service: %s", s.Name)
		}

		// Assert internal state equals consul's state
		cs, ok := consulServices[s.ID]
		if !ok {
			t.Errorf("service not in consul: %s id: %s", s.Name, s.ID)
			continue
		}
		if !reflect.DeepEqual(s.Tags, cs.Tags) {
			t.Errorf("mismatched tags in syncer state and consul for %s:\nsyncer: %v\nconsul: %v", s.Name, s.Tags, cs.Tags)
		}
		if cs.Port != s.Port {
			t.Errorf("mismatched port in syncer state and consul for %s\nsyncer: %v\nconsul: %v", s.Name, s.Port, cs.Port)
		}
		if cs.Address != s.Address {
			t.Errorf("mismatched address in syncer state and consul for %s\nsyncer: %v\nconsul: %v", s.Name, s.Address, cs.Address)
		}
	}
	if found != 3 {
		t.Fatalf("expected 3 services locally but found %d", found)
	}
}
