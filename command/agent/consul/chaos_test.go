// +build chaos

package consul

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

func TestSyncerChaos(t *testing.T) {
	// Create an embedded Consul server
	testconsul := testutil.NewTestServerConfig(t, func(c *testutil.TestServerConfig) {
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		}
	})
	defer testconsul.Stop()

	// Configure Syncer to talk to the test server
	cconf := config.DefaultConsulConfig()
	cconf.Addr = testconsul.HTTPAddr

	clientSyncer, err := NewSyncer(cconf, nil, logger)
	if err != nil {
		t.Fatalf("Error creating Syncer: %v", err)
	}
	defer clientSyncer.Shutdown()

	execSyncer, err := NewSyncer(cconf, nil, logger)
	if err != nil {
		t.Fatalf("Error creating Syncer: %v", err)
	}
	defer execSyncer.Shutdown()

	clientService := &structs.Service{Name: "nomad-client"}
	services := map[ServiceKey]*structs.Service{
		GenerateServiceKey(clientService): clientService,
	}
	if err := clientSyncer.SetServices("client", services); err != nil {
		t.Fatalf("error setting client service: %v", err)
	}

	const execn = 100
	const reapern = 2
	errors := make(chan error, 100)
	wg := sync.WaitGroup{}

	// Start goroutines to concurrently SetServices
	for i := 0; i < execn; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			domain := ServiceDomain(fmt.Sprintf("exec-%d", i))
			services := map[ServiceKey]*structs.Service{}
			for ii := 0; ii < 10; ii++ {
				s := &structs.Service{Name: fmt.Sprintf("exec-%d-%d", i, ii)}
				services[GenerateServiceKey(s)] = s
				if err := execSyncer.SetServices(domain, services); err != nil {
					select {
					case errors <- err:
					default:
					}
					return
				}
				time.Sleep(1)
			}
		}(i)
	}

	// SyncServices runs a timer started by Syncer.Run which we don't use
	// in this test, so run SyncServices concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < execn; i++ {
			if err := execSyncer.SyncServices(); err != nil {
				select {
				case errors <- err:
				default:
				}
				return
			}
			time.Sleep(100)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := clientSyncer.ReapUnmatched([]ServiceDomain{"nomad-client"}); err != nil {
			select {
			case errors <- err:
			default:
			}
			return
		}
	}()

	// Reap all but exec-0-*
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < execn; i++ {
			if err := execSyncer.ReapUnmatched([]ServiceDomain{"exec-0", ServiceDomain(fmt.Sprintf("exec-%d", i))}); err != nil {
				select {
				case errors <- err:
				default:
				}
			}
			time.Sleep(100)
		}
	}()

	go func() {
		wg.Wait()
		close(errors)
	}()

	for err := range errors {
		if err != nil {
			t.Errorf("error setting service from executor goroutine: %v", err)
		}
	}

	// Do a final ReapUnmatched to get consul back into a deterministic state
	if err := execSyncer.ReapUnmatched([]ServiceDomain{"exec-0"}); err != nil {
		t.Fatalf("error doing final reap: %v", err)
	}

	// flattenedServices should be fully populated as ReapUnmatched doesn't
	// touch Syncer's internal state
	expected := map[string]struct{}{}
	for i := 0; i < execn; i++ {
		for ii := 0; ii < 10; ii++ {
			expected[fmt.Sprintf("exec-%d-%d", i, ii)] = struct{}{}
		}
	}

	for _, s := range execSyncer.flattenedServices() {
		_, ok := expected[s.Name]
		if !ok {
			t.Errorf("%s unexpected", s.Name)
		}
		delete(expected, s.Name)
	}
	if len(expected) > 0 {
		left := []string{}
		for s := range expected {
			left = append(left, s)
		}
		sort.Strings(left)
		t.Errorf("Couldn't find %d names in flattened services:\n%s", len(expected), strings.Join(left, "\n"))
	}

	// All but exec-0 and possibly some of exec-99 should have been reaped
	{
		services, err := execSyncer.client.Agent().Services()
		if err != nil {
			t.Fatalf("Error getting services: %v", err)
		}
		expected := []int{}
		for k, service := range services {
			if service.Service == "consul" {
				continue
			}
			i := -1
			ii := -1
			fmt.Sscanf(service.Service, "exec-%d-%d", &i, &ii)
			switch {
			case i == -1 || ii == -1:
				t.Errorf("invalid service: %s -> %s", k, service.Service)
			case i != 0 || ii > 9:
				t.Errorf("unexpected service: %s -> %s", k, service.Service)
			default:
				expected = append(expected, ii)
			}
		}
		if len(expected) != 10 {
			t.Errorf("expected 0-9 but found: %#q", expected)
		}
	}
}
