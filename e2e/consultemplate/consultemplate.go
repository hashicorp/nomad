package consultemplate

import (
	"fmt"
	"os"
	"strings"
	"time"

	capi "github.com/hashicorp/consul/api"
	api "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/nomad/structs"
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
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *ConsulTemplateTest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIDs {
		_, err := e2eutil.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err, "could not clean up job", id)
	}
	tc.jobIDs = []string{}

	for _, key := range tc.consulKeys {
		_, err := tc.Consul().KV().Delete(key, nil)
		f.Assert().NoError(err, "could not clean up consul key", key)
	}
	tc.consulKeys = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestTemplateUpdateTriggers exercises consul-template integration, verifying that:
// - missing keys block allocations from starting
// - key updates trigger re-render
// - service updates trigger re-render
// - 'noop' vs ''restart' configuration
func (tc *ConsulTemplateTest) TestTemplateUpdateTriggers(f *framework.F) {

	wc := &e2eutil.WaitConfig{}
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
	allocs, err := e2eutil.AllocsForJob(jobID, ns)
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
			out, err := e2eutil.Command("nomad", "alloc", "status", allocID)
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
			out, err := e2eutil.Command("nomad", "alloc", "status", allocID)
			f.NoError(err, "could not get allocation status")

			section, err := e2eutil.GetSection(out, "Task Events:")
			f.NoError(err, out)

			restarts, err := e2eutil.GetField(section, "Total Restarts")
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
		out, err := e2eutil.Command("nomad", "alloc", "status", allocID)
		f.NoError(err, "could not get allocation status")

		section, err := e2eutil.GetSection(out, "Task Events:")
		f.NoError(err, out)

		restarts, err := e2eutil.GetField(section, "Total Restarts")
		f.NoError(err)
		f.Equal("1", restarts, "expected no new restarts for group")
	}
}

// TestTemplatePathInterpolation_Ok asserts that NOMAD_*_DIR variables are
// properly interpolated into template source and destination paths without
// being treated as escaping.
func (tc *ConsulTemplateTest) TestTemplatePathInterpolation_Ok(f *framework.F) {
	jobID := "template-paths-" + uuid.Generate()[:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocStubs := e2eutil.RegisterAndWaitForAllocs(
		f.T(), tc.Nomad(), "consultemplate/input/template_paths.nomad", jobID, "")
	f.Len(allocStubs, 1)
	allocID := allocStubs[0].ID

	e2eutil.WaitForAllocRunning(f.T(), tc.Nomad(), allocID)

	f.NoError(waitForTemplateRender(allocID, "task/secrets/foo/dst",
		func(out string) bool {
			return len(out) > 0
		}, nil), "expected file to have contents")

	f.NoError(waitForTemplateRender(allocID, "alloc/shared.txt",
		func(out string) bool {
			return len(out) > 0
		}, nil), "expected shared-alloc-dir file to have contents")
}

// TestTemplatePathInterpolation_Bad asserts that template.source paths are not
// allowed to escape the sandbox directory tree by default.
func (tc *ConsulTemplateTest) TestTemplatePathInterpolation_Bad(f *framework.F) {
	wc := &e2eutil.WaitConfig{}
	interval, retries := wc.OrDefault()

	jobID := "bad-template-paths-" + uuid.Generate()[:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocStubs := e2eutil.RegisterAndWaitForAllocs(
		f.T(), tc.Nomad(), "consultemplate/input/bad_template_paths.nomad", jobID, "")
	f.Len(allocStubs, 1)
	allocID := allocStubs[0].ID

	// Wait for alloc to fail
	var err error
	var alloc *api.Allocation
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		alloc, _, err = tc.Nomad().Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		return alloc.ClientStatus == structs.AllocClientStatusFailed, fmt.Errorf("expected status failed, but was: %s", alloc.ClientStatus)
	}, func(err error) {
		f.NoError(err, "failed to wait on alloc")
	})

	// Assert the "source escapes" error occurred to prevent false
	// positives.
	found := false
	for _, event := range alloc.TaskStates["task"].Events {
		if strings.Contains(event.DisplayMessage, "template source path escapes alloc directory") {
			found = true
			break
		}
	}
	f.True(found, "alloc failed but NOT due to expected source path escape error")
}

// TestTemplatePathInterpolation_SharedAllocDir asserts that NOMAD_ALLOC_DIR
// is supported as a destination for artifact and template blocks, and
// that it is properly interpolated for task drivers with varying
// filesystem isolation
func (tc *ConsulTemplateTest) TestTemplatePathInterpolation_SharedAllocDir(f *framework.F) {
	jobID := "template-shared-alloc-" + uuid.Generate()[:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocStubs := e2eutil.RegisterAndWaitForAllocs(
		f.T(), tc.Nomad(), "consultemplate/input/template_shared_alloc.nomad", jobID, "")
	f.Len(allocStubs, 1)
	allocID := allocStubs[0].ID

	e2eutil.WaitForAllocRunning(f.T(), tc.Nomad(), allocID)

	for _, task := range []string{"docker", "exec", "raw_exec"} {

		// tests that we can render templates into the shared alloc directory
		f.NoError(waitForTaskFile(allocID, task, "${NOMAD_ALLOC_DIR}/raw_exec.env",
			func(out string) bool {
				return len(out) > 0 && strings.TrimSpace(out) != "/alloc"
			}, nil), "expected raw_exec.env to not be '/alloc'")

		f.NoError(waitForTaskFile(allocID, task, "${NOMAD_ALLOC_DIR}/exec.env",
			func(out string) bool {
				return strings.TrimSpace(out) == "/alloc"
			}, nil), "expected shared exec.env to contain '/alloc'")

		f.NoError(waitForTaskFile(allocID, task, "${NOMAD_ALLOC_DIR}/docker.env",
			func(out string) bool {
				return strings.TrimSpace(out) == "/alloc"
			}, nil), "expected shared docker.env to contain '/alloc'")

		// test that we can fetch artifacts into the shared alloc directory
		for _, a := range []string{"google1.html", "google2.html", "google3.html"} {
			f.NoError(waitForTaskFile(allocID, task, "${NOMAD_ALLOC_DIR}/"+a,
				func(out string) bool {
					return len(out) > 0
				}, nil), "expected artifact in alloc dir")
		}

		// test that we can load environment variables rendered with templates using interpolated paths
		out, err := e2eutil.Command("nomad", "alloc", "exec", "-task", task, allocID, "sh", "-c", "env")
		f.NoError(err)
		f.Contains(out, "HELLO_FROM=raw_exec")
	}
}

func waitForTaskFile(allocID, task, path string, test func(out string) bool, wc *e2eutil.WaitConfig) error {
	var err error
	var out string
	interval, retries := wc.OrDefault()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err = e2eutil.Command("nomad", "alloc", "exec", "-task", task, allocID, "sh", "-c", "cat "+path)
		if err != nil {
			return false, fmt.Errorf("could not cat file %q from task %q in allocation %q: %v",
				path, task, allocID, err)
		}
		return test(out), nil
	}, func(e error) {
		err = fmt.Errorf("test for file content failed: got %#v\nerror: %v", out, e)
	})
	return err
}

// waitForTemplateRender is a helper that grabs a file via alloc fs
// and tests it for
func waitForTemplateRender(allocID, path string, test func(string) bool, wc *e2eutil.WaitConfig) error {
	var err error
	var out string
	interval, retries := wc.OrDefault()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err = e2eutil.Command("nomad", "alloc", "fs", allocID, path)
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
func waitForAllocStatusByGroup(jobID, ns string, expected map[string]string, wc *e2eutil.WaitConfig) error {
	var got []map[string]string
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		got, err = e2eutil.AllocsForJob(jobID, ns)
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
