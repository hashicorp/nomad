package consultemplate

import (
	"fmt"
	"os"
	"strings"
	"time"

	capi "github.com/hashicorp/consul/api"
	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/testutil"
)

const ns = ""

type ConsulTemplateTest struct {
	framework.TC
	jobIDs     []string
	consulKeys []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "ConsulTemplate",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(ConsulTemplateTest),
		},
	})
}

func (tc *ConsulTemplateTest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *ConsulTemplateTest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIDs {
		_, err := e2e.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err, "could not clean up job", id)
	}
	tc.jobIDs = []string{}

	for _, key := range tc.consulKeys {
		_, err := tc.Consul().KV().Delete(key, nil)
		f.Assert().NoError(err, "could not clean up consul key", key)
	}
	tc.consulKeys = []string{}

	_, err := e2e.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestTemplateUpdateTriggers exercises consul-template integration, verifying that:
// - missing keys block allocations from starting
// - key updates trigger re-render
// - service updates trigger re-render
// - 'noop' vs ''restart' configuration
func (tc *ConsulTemplateTest) TestTemplateUpdateTriggers(f *framework.F) {

	wc := &e2e.WaitConfig{}
	interval, retries := wc.OrDefault()

	key := "consultemplate-" + uuid.Generate()[:8]
	jobID := key

	replacement := fmt.Sprintf(`---
key: {{ key "%s" }}
job: {{ env "NOMAD_JOB_NAME" }}
`, key)

	// Ensure consul key does not exist
	_, err := tc.Consul().KV().Delete(key, nil)
	f.NoError(err)

	// Parse job so we can replace the template stanza with isolated keys
	job, err := jobspec.ParseFile("consultemplate/input/templating.nomad")
	f.NoError(err)
	job.ID = &jobID

	job.TaskGroups[0].Tasks[0].Templates[1].EmbeddedTmpl = &replacement
	job.TaskGroups[1].Tasks[0].Templates[1].EmbeddedTmpl = &replacement

	tc.jobIDs = append(tc.jobIDs, jobID)

	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register job")

	expected := map[string]string{
		"upstream":          "running",
		"exec_downstream":   "pending",
		"docker_downstream": "pending"}
	f.NoError(waitForAllocStatusByGroup(jobID, ns, expected, nil))

	// We won't reschedule any of these allocs, so we can cache these IDs for later
	downstreams := map[string]string{} // alloc ID -> group name
	allocs, err := e2e.AllocsForJob(jobID, ns)
	f.NoError(err)
	for _, alloc := range allocs {
		group := alloc["Task Group"]
		if group == "docker_downstream" || group == "exec_downstream" {
			downstreams[alloc["ID"]] = group
		}
	}

	// note: checking pending above doesn't tell us whether we've tried to render
	// the template yet, so we still need to poll for the template event
	for allocID, group := range downstreams {
		var checkErr error
		testutil.WaitForResultRetries(retries, func() (bool, error) {
			time.Sleep(interval)
			out, err := e2e.Command("nomad", "alloc", "status", allocID)
			f.NoError(err, "could not get allocation status")
			return strings.Contains(out, "Missing: kv.block"),
				fmt.Errorf("expected %q to be blocked on Consul key", group)
		}, func(e error) {
			checkErr = e
		})
		f.NoError(checkErr)
	}

	// Write our key to Consul
	_, err = tc.Consul().KV().Put(&capi.KVPair{Key: key, Value: []byte("foo")}, nil)
	f.NoError(err)
	tc.consulKeys = append(tc.consulKeys, key)

	// template will render, allowing downstream allocs to run
	expected = map[string]string{
		"upstream":          "running",
		"exec_downstream":   "running",
		"docker_downstream": "running"}
	f.NoError(waitForAllocStatusByGroup(jobID, ns, expected, nil))

	// verify we've rendered the templates
	for allocID := range downstreams {
		f.NoError(waitForTemplateRender(allocID, "task/local/kv.yml",
			func(out string) bool {
				return strings.TrimSpace(out) == "---\nkey: foo\njob: templating"
			}, nil), "expected consul key to be rendered")

		f.NoError(waitForTemplateRender(allocID, "task/local/services.conf",
			func(out string) bool {
				confLines := strings.Split(strings.TrimSpace(out), "\n")
				servers := 0
				for _, line := range confLines {
					if strings.HasPrefix(line, "server upstream-service ") {
						servers++
					}
				}
				return servers == 2
			}, nil), "expected 2 upstream servers")
	}

	// Update our key in Consul
	_, err = tc.Consul().KV().Put(&capi.KVPair{Key: key, Value: []byte("bar")}, nil)
	f.NoError(err)

	// Wait for restart
	for allocID, group := range downstreams {
		var checkErr error
		testutil.WaitForResultRetries(retries, func() (bool, error) {
			time.Sleep(interval)
			out, err := e2e.Command("nomad", "alloc", "status", allocID)
			f.NoError(err, "could not get allocation status")

			section, err := e2e.GetSection(out, "Task Events:")
			f.NoError(err, out)

			restarts, err := e2e.GetField(section, "Total Restarts")
			f.NoError(err)
			return restarts == "1",
				fmt.Errorf("expected 1 restart for %q but found %s", group, restarts)
		}, func(e error) {
			checkErr = e
		})
		f.NoError(checkErr)

		// verify we've re-rendered the template
		f.NoError(waitForTemplateRender(allocID, "task/local/kv.yml",
			func(out string) bool {
				return strings.TrimSpace(out) == "---\nkey: bar\njob: templating"
			}, nil), "expected updated consul key")
	}

	// increase the count for upstreams
	count := 3
	job.TaskGroups[2].Count = &count
	_, _, err = tc.Nomad().Jobs().Register(job, nil)
	f.NoError(err, "could not register job")

	// wait for re-rendering
	for allocID := range downstreams {
		f.NoError(waitForTemplateRender(allocID, "task/local/services.conf",
			func(out string) bool {
				confLines := strings.Split(strings.TrimSpace(out), "\n")
				servers := 0
				for _, line := range confLines {
					if strings.HasPrefix(line, "server upstream-service ") {
						servers++
					}
				}
				return servers == 3
			}, nil), "expected 3 upstream servers")

		// verify noop was honored: no additional restarts
		out, err := e2e.Command("nomad", "alloc", "status", allocID)
		f.NoError(err, "could not get allocation status")

		section, err := e2e.GetSection(out, "Task Events:")
		f.NoError(err, out)

		restarts, err := e2e.GetField(section, "Total Restarts")
		f.NoError(err)
		f.Equal("1", restarts, "expected no new restarts for group")
	}
}

// waitForTemplateRender is a helper that grabs a file via alloc fs
// and tests it for
func waitForTemplateRender(allocID, path string, test func(string) bool, wc *e2e.WaitConfig) error {
	var err error
	var out string
	interval, retries := wc.OrDefault()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err = e2e.Command("nomad", "alloc", "fs", allocID, path)
		if err != nil {
			return false, fmt.Errorf("could not get file %q from allocation %q: %v",
				path, allocID, err)
		}
		return test(out), nil
	}, func(e error) {
		err = fmt.Errorf("test for file content failed: got %#v\nerror: %v", out, e)
	})
	return err
}

// waitForAllocStatusByGroup is similar to WaitForAllocStatus but maps
// specific task group names to statuses without having to deal with specific counts
func waitForAllocStatusByGroup(jobID, ns string, expected map[string]string, wc *e2e.WaitConfig) error {
	var got []map[string]string
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		got, err = e2e.AllocsForJob(jobID, ns)
		if err != nil {
			return false, err
		}
		for _, row := range got {
			group := row["Task Group"]
			expectedStatus := expected[group]
			gotStatus := row["Status"]
			if expectedStatus != gotStatus {
				return false, fmt.Errorf("expected %q to be %q, got %q",
					group, expectedStatus, gotStatus)
			}
		}
		err = nil
		return true, nil
	}, func(e error) {
		err = fmt.Errorf("alloc status check failed: got %#v\nerror: %v", got, e)
	})
	return err
}
