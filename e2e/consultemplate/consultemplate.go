// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	// namespaceIDs tracks the created namespace for removal after test
	// completion.
	namespaceIDs []string

	// namespacedJobIDs tracks any non-default namespaced jobs for removal
	// after test completion.
	namespacedJobIDs map[string][]string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "ConsulTemplate",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			&ConsulTemplateTest{
				namespacedJobIDs: make(map[string][]string),
			},
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
		err := e2eutil.StopJob(id, "-purge")
		f.Assert().NoError(err, "could not clean up job", id)
	}
	tc.jobIDs = []string{}

	for _, key := range tc.consulKeys {
		_, err := tc.Consul().KV().Delete(key, nil)
		f.Assert().NoError(err, "could not clean up consul key", key)
	}
	tc.consulKeys = []string{}

	for namespace, jobIDs := range tc.namespacedJobIDs {
		for _, jobID := range jobIDs {
			err := e2eutil.StopJob(jobID, "-purge", "-namespace", namespace)
			f.Assert().NoError(err)
		}
	}
	tc.namespacedJobIDs = make(map[string][]string)

	for _, ns := range tc.namespaceIDs {
		_, err := e2eutil.Command("nomad", "namespace", "delete", ns)
		f.Assert().NoError(err)
	}
	tc.namespaceIDs = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestTemplateUpdateTriggers exercises consul-template integration, verifying that:
// - missing keys block allocations from starting
// - key updates trigger re-render
// - service updates trigger re-render
// - 'noop' vs ”restart' configuration
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

	// Parse job so we can replace the template block with isolated keys
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
	f.NoError(waitForAllocStatusByGroup(jobID, ns, expected, &e2eutil.WaitConfig{
		Interval: time.Millisecond * 300,
		Retries:  100,
	}))

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

// TestConsulTemplate_NomadServiceLookups tests consul-templates Nomad service
// lookup functionality. It runs a job which registers two services, then
// another which performs both a list and read template function lookup against
// registered services.
func (tc *ConsulTemplateTest) TestConsulTemplate_NomadServiceLookups(f *framework.F) {

	// Set up our base job that will be used in various manners.
	serviceJob, err := jobspec.ParseFile("consultemplate/input/nomad_provider_service.nomad")
	f.NoError(err)
	serviceJobID := "test-consul-template-nomad-lookups" + uuid.Generate()[0:8]
	serviceJob.ID = &serviceJobID

	_, _, err = tc.Nomad().Jobs().Register(serviceJob, nil)
	f.NoError(err)
	tc.jobIDs = append(tc.jobIDs, serviceJobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(serviceJobID, "default", []string{"running"}), "job should be running")

	// Pull the allocation ID for the job, we use this to ensure this is found
	// in the rendered template later on.
	serviceJobAllocs, err := e2eutil.AllocsForJob(serviceJobID, "default")
	f.NoError(err)
	f.Len(serviceJobAllocs, 1)
	serviceAllocID := serviceJobAllocs[0]["ID"]

	// Create at non-default namespace.
	_, err = e2eutil.Command("nomad", "namespace", "apply", "platform")
	f.NoError(err)
	tc.namespaceIDs = append(tc.namespaceIDs, "NamespaceA")

	// Register a job which includes services destined for the Nomad provider
	// into the platform namespace. This is used to ensure consul-template
	// lookups stay bound to the allocation namespace.
	diffNamespaceServiceJobID := "test-consul-template-nomad-lookups" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(diffNamespaceServiceJobID, "consultemplate/input/nomad_provider_service_ns.nomad"))
	tc.namespacedJobIDs["platform"] = append(tc.namespacedJobIDs["platform"], diffNamespaceServiceJobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(diffNamespaceServiceJobID, "platform", []string{"running"}), "job should be running")

	// Register a job which includes consul-template function performing Nomad
	// service listing and reads.
	serviceLookupJobID := "test-consul-template-nomad-lookups" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(serviceLookupJobID, "consultemplate/input/nomad_provider_service_lookup.nomad"))
	tc.jobIDs = append(tc.jobIDs, serviceLookupJobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(serviceLookupJobID, "default", []string{"running"}), "job should be running")

	// Find the allocation ID for the job which contains templates, so we can
	// perform filesystem actions.
	serviceLookupJobAllocs, err := e2eutil.AllocsForJob(serviceLookupJobID, "default")
	f.NoError(err)
	f.Len(serviceLookupJobAllocs, 1)
	serviceLookupAllocID := serviceLookupJobAllocs[0]["ID"]

	// Ensure the listing (nomadServices) template function has found all
	// services within the default namespace.
	err = waitForTaskFile(serviceLookupAllocID, "test", "${NOMAD_TASK_DIR}/services.conf",
		func(out string) bool {
			if !strings.Contains(out, "service default-nomad-provider-service-primary [bar foo]") {
				return false
			}
			if !strings.Contains(out, "service default-nomad-provider-service-secondary [baz buz]") {
				return false
			}
			return !strings.Contains(out, "service platform-nomad-provider-service-secondary [baz buz]")
		}, nil)
	f.NoError(err)

	// Ensure the direct service lookup has found the entry we expect.
	err = waitForTaskFile(serviceLookupAllocID, "test", "${NOMAD_TASK_DIR}/service.conf",
		func(out string) bool {
			expected := fmt.Sprintf("service default-nomad-provider-service-primary [bar foo] dc1 %s", serviceAllocID)
			return strings.Contains(out, expected)
		}, nil)
	f.NoError(err)

	// Scale the default namespaced service job in order to change the expected
	// number of entries.
	count := 3
	serviceJob.TaskGroups[0].Count = &count
	_, _, err = tc.Nomad().Jobs().Register(serviceJob, nil)
	f.NoError(err)

	// Pull the allocation ID for the job, we use this to ensure this is found
	// in the rendered template later on. Wrap this in an eventual do to the
	// eventual consistency around the service registration process.
	f.Eventually(func() bool {
		serviceJobAllocs, err = e2eutil.AllocsForJob(serviceJobID, "default")
		if err != nil {
			return false
		}
		return len(serviceJobAllocs) == 3
	}, 10*time.Second, 200*time.Millisecond, "unexpected number of allocs found")

	// Track the expected entries, including the allocID to make this test
	// actually valuable.
	var expectedEntries []string
	for _, allocs := range serviceJobAllocs {
		e := fmt.Sprintf("service default-nomad-provider-service-primary [bar foo] dc1 %s", allocs["ID"])
		expectedEntries = append(expectedEntries, e)
	}

	// Ensure the direct service lookup has the new entries we expect.
	err = waitForTaskFile(serviceLookupAllocID, "test", "${NOMAD_TASK_DIR}/service.conf",
		func(out string) bool {
			for _, entry := range expectedEntries {
				if !strings.Contains(out, entry) {
					return false
				}
			}
			return true
		}, nil)
	f.NoError(err)
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
