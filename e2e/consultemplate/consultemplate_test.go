// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consultemplate

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/e2e/v3/namespaces3"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const ns = ""

// TestTemplateUpdateTriggers exercises consul-template integration, verifying that:
// - missing keys block allocations from starting
// - key updates trigger re-render
// - service updates trigger re-render
// - 'noop' vs 'restart' configuration
func TestTemplateUpdateTriggers(t *testing.T) {

	// use a random suffix to encapsulate test keys and job ID for cleanup
	key := "consultemplate-" + uuid.Generate()[:8]

	cc := e2eutil.ConsulClient(t)

	// Ensure consul key does not exist
	_, err := cc.KV().Delete(key, nil)
	must.NoError(t, err)

	cleanupGC(t)

	submission, cleanupJob := jobs3.Submit(t,
		"./input/update_triggers.nomad.hcl",
		jobs3.Detach(),
		jobs3.ReplaceInJobSpec("consultemplatetest", key),
	)
	t.Cleanup(cleanupJob)
	jobID := submission.JobID()

	cleanupKey(t, key)

	expected := map[string]string{
		"upstream":          "running",
		"exec_downstream":   "pending",
		"docker_downstream": "pending"}

	mustWaitForStatusByGroup(t, jobID, ns, expected, 20*time.Second)

	// We won't reschedule any of these allocs, so we can cache these IDs for later
	downstreams := map[string]string{} // alloc ID -> group name
	allocs, err := e2eutil.AllocsForJob(jobID, ns)
	must.NoError(t, err)
	for _, alloc := range allocs {
		group := alloc["Task Group"]
		if group == "docker_downstream" || group == "exec_downstream" {
			downstreams[alloc["ID"]] = group
			t.Logf("alloc %q (%s) on node %q", alloc["ID"], group, alloc["Node ID"])
		}
	}

	now := time.Now()
	for allocID, group := range downstreams {

		// note: checking pending above doesn't tell us whether we've tried to
		// render the KV template yet, so we still need to poll for the template
		// event; note that it may take a long while for the `exec` task to be
		// ready
		must.Wait(t, wait.InitialSuccess(
			wait.BoolFunc(func() bool {
				out, err := e2eutil.Command("nomad", "alloc", "status", allocID)
				must.NoError(t, err)
				return strings.Contains(out, "Missing: kv.block")
			}),
			wait.Gap(time.Millisecond*500),
			wait.Timeout(time.Second*30),
		), must.Sprintf("expected %q to be blocked on Consul key", group))

		// note: although the tasks are stuck in pending, the service templates
		// should be rendered already by this point or quickly thereafter
		t.Logf("verifying service template contents")
		mustWaitTemplateRender(t, allocID, "task/local/services.conf",
			func(out string) error {
				confLines := strings.Split(strings.TrimSpace(out), "\n")
				servers := 0
				for _, line := range confLines {
					if strings.HasPrefix(line, "server upstream-service ") {
						servers++
					}
				}
				if servers != 2 {
					return fmt.Errorf(
						"expected 2 upstream servers for alloc %q, got:\n%s", allocID, out)
				}
				return nil
			},
			time.Second*5,
		)

		t.Logf("ok for alloc %q: elapsed=%v", allocID, time.Since(now))
	}

	// Write our key to Consul
	_, err = cc.KV().Put(&capi.KVPair{Key: key, Value: []byte("foo")}, nil)
	must.NoError(t, err)

	t.Logf("waiting for blocked downstream allocs to run")

	// template will render, allowing downstream allocs to run
	expected = map[string]string{
		"upstream":          "running",
		"exec_downstream":   "running",
		"docker_downstream": "running"}
	mustWaitForStatusByGroup(t, jobID, ns, expected, 30*time.Second)

	for allocID := range downstreams {

		// verify we've rendered the templates
		t.Logf("verifying kv template contents")
		mustWaitTemplateRender(t, allocID, "task/local/kv.yml",
			func(out string) error {
				if strings.TrimSpace(out) != "---\nkey: foo\njob: templating" {
					fmt.Errorf("expected consul key to be rendered for alloc %q, got:%s", allocID, out)
				}
				return nil
			},
			time.Second*10)
	}

	// Update our key in Consul
	t.Logf("updating key %v", key)
	_, err = cc.KV().Put(&capi.KVPair{Key: key, Value: []byte("bar")}, nil)
	must.NoError(t, err)

	// Wait for restart
	t.Logf("waiting for restart")
	for allocID, group := range downstreams {
		must.Wait(t, wait.InitialSuccess(
			wait.BoolFunc(func() bool {
				out, err := e2eutil.Command("nomad", "alloc", "status", allocID)
				must.NoError(t, err)

				section, err := e2eutil.GetSection(out, "Task Events:")
				must.NoError(t, err,
					must.Sprintf("could not parse Task Events section from: %v", out))

				restarts, err := e2eutil.GetField(section, "Total Restarts")
				must.NoError(t, err)
				return restarts == "1"
			}),
			wait.Gap(time.Millisecond*500),
			wait.Timeout(time.Second*20),
		), must.Sprintf("expected 1 restart for %q", group))
	}

	t.Logf("waiting for template re-render")
	for allocID := range downstreams {
		// verify we've re-rendered the template
		mustWaitTemplateRender(t, allocID, "task/local/kv.yml",
			func(out string) error {
				if strings.TrimSpace(out) != "---\nkey: bar\njob: templating" {
					fmt.Errorf("expected updated consul key for alloc %q, got:%s", allocID, out)
				}
				return nil
			},
			time.Second*10)
	}

	// increase the count for upstreams
	t.Logf("increasing upstream count")
	submission.Rerun(jobs3.MutateJobSpec(func(spec string) string {
		return strings.Replace(spec, "count = 2", "count = 3", 1)
	}))

	// wait for re-rendering
	now = time.Now()
	t.Logf("waiting for service template re-render")
	for allocID := range downstreams {
		mustWaitTemplateRender(t, allocID, "task/local/services.conf",
			func(out string) error {
				confLines := strings.Split(strings.TrimSpace(out), "\n")
				servers := 0
				for _, line := range confLines {
					if strings.HasPrefix(line, "server upstream-service ") {
						servers++
					}
				}
				if servers != 3 {
					return fmt.Errorf(
						"expected 3 upstream servers for alloc %q, got:\n%s", allocID, out)
				}
				return nil
			},
			time.Second*30,
		)
		t.Logf("ok for alloc %q: elapsed=%v", allocID, time.Since(now))
	}

	t.Logf("verifying no restart")
	for allocID := range downstreams {

		// verify noop was honored: no additional restarts
		out, err := e2eutil.Command("nomad", "alloc", "status", allocID)
		must.NoError(t, err, must.Sprint("could not get allocation status"))

		section, err := e2eutil.GetSection(out, "Task Events:")
		must.NoError(t, err, must.Sprintf("could not parse Task Events from: %v", out))

		restarts, err := e2eutil.GetField(section, "Total Restarts")
		must.NoError(t, err)
		must.Eq(t, "1", restarts, must.Sprint("expected no new restarts for group"))
	}
}

// TestTemplatePathInterpolation_Ok asserts that NOMAD_*_DIR variables are
// properly interpolated into template source and destination paths without
// being treated as escaping.
func TestTemplatePathInterpolation_Ok(t *testing.T) {
	cleanupGC(t)
	submission, cleanupJob := jobs3.Submit(t, "./input/template_paths.nomad")
	t.Cleanup(cleanupJob)
	allocID := submission.AllocID("template-paths")

	mustWaitTemplateRender(t, allocID, "task/secrets/foo/dst",
		func(out string) error {
			if len(out) == 0 {
				return fmt.Errorf("expected file to have contents")
			}
			return nil
		},
		time.Second*10)

	mustWaitTemplateRender(t, allocID, "alloc/shared.txt",
		func(out string) error {
			if len(out) == 0 {
				return fmt.Errorf("expected shared-alloc-dir file to have contents")
			}
			return nil
		},
		time.Second*10)
}

// TestTemplatePathInterpolation_Bad asserts that template.source paths are not
// allowed to escape the sandbox directory tree by default.
func TestTemplatePathInterpolation_Bad(t *testing.T) {
	cleanupGC(t)
	submission, cleanupJob := jobs3.Submit(t,
		"./input/bad_template_paths.nomad",
		jobs3.Detach(),
	)
	t.Cleanup(cleanupJob)
	allocID := submission.AllocID("template-paths")

	nc := e2eutil.NomadClient(t)

	// Wait for alloc to fail
	var err error
	var alloc *api.Allocation

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			alloc, _, err = nc.Allocations().Info(allocID, nil)
			must.NoError(t, err)
			return alloc.ClientStatus == structs.AllocClientStatusFailed
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(500*time.Millisecond),
	), must.Sprint("expected failed alloc"))

	// Assert the "source escapes" error occurred to prevent false
	// positives.
	found := false
	for _, event := range alloc.TaskStates["task"].Events {
		if strings.Contains(event.DisplayMessage, "template source path escapes alloc directory") {
			found = true
			break
		}
	}
	must.True(t, found, must.Sprint("alloc failed but NOT due to expected source path escape error"))
}

// TestTemplatePathInterpolation_SharedAllocDir asserts that NOMAD_ALLOC_DIR
// is supported as a destination for artifact and template blocks, and
// that it is properly interpolated for task drivers with varying
// filesystem isolation
func TestTemplatePathInterpolation_SharedAllocDir(t *testing.T) {
	cleanupGC(t)
	submission, cleanupJob := jobs3.Submit(t,
		"./input/template_shared_alloc.nomad",
		jobs3.Timeout(time.Second*60)) // note: exec tasks can take a while
	t.Cleanup(cleanupJob)
	allocID := submission.AllocID("template-paths")

	for _, task := range []string{"docker", "exec", "raw_exec"} {

		// tests that we can render templates into the shared alloc directory
		mustWaitForTaskFile(t, allocID, task, "${NOMAD_ALLOC_DIR}/raw_exec.env",
			func(out string) error {
				if len(out) == 0 || strings.TrimSpace(out) == "/alloc" {
					return fmt.Errorf("expected raw_exec.env to not be '/alloc'")
				}
				return nil
			})

		mustWaitForTaskFile(t, allocID, task, "${NOMAD_ALLOC_DIR}/exec.env",
			func(out string) error {
				if strings.TrimSpace(out) != "/alloc" {
					return fmt.Errorf("expected shared exec.env to contain '/alloc'")
				}
				return nil
			})

		mustWaitForTaskFile(t, allocID, task, "${NOMAD_ALLOC_DIR}/docker.env",
			func(out string) error {
				if strings.TrimSpace(out) != "/alloc" {
					return fmt.Errorf("expected shared docker.env to contain '/alloc'")
				}
				return nil
			})

		// test that we can fetch artifacts into the shared alloc directory
		for _, a := range []string{"google1.html", "google2.html", "google3.html"} {
			mustWaitForTaskFile(t, allocID, task, "${NOMAD_ALLOC_DIR}/"+a,
				func(out string) error {
					if len(out) == 0 {
						return fmt.Errorf("expected artifact in alloc dir")
					}
					return nil
				})
		}

		// test that we can load environment variables rendered with templates using interpolated paths
		out, err := e2eutil.Command("nomad", "alloc", "exec", "-task", task, allocID, "sh", "-c", "env")
		must.NoError(t, err)
		must.StrContains(t, out, "HELLO_FROM=raw_exec")
	}
}

// TestConsulTemplate_NomadServiceLookups tests consul-templates Nomad service
// lookup functionality. It runs a job which registers two services, then
// another which performs both a list and read template function lookup against
// registered services.
func TestConsulTemplate_NomadServiceLookups(t *testing.T) {

	cleanupGC(t)

	// The service job will be the source for template data
	serviceJobSubmission, cleanupJob := jobs3.Submit(t, "./input/nomad_provider_service.nomad")
	t.Cleanup(cleanupJob)
	serviceAllocID := serviceJobSubmission.AllocID("nomad_provider_service")

	// Create a non-default namespace.
	t.Cleanup(namespaces3.Create(t, "platform"))

	// Register a job which includes services destined for the Nomad provider
	// into the platform namespace. This is used to ensure consul-template
	// lookups stay bound to the allocation namespace.
	_, diffCleanupJob := jobs3.Submit(t, "./input/nomad_provider_service_ns.nomad",
		jobs3.Namespace("platform"))
	t.Cleanup(diffCleanupJob)

	// Register a job which includes consul-template function performing Nomad
	// service listing and reads.
	serviceLookupJobSubmission, serviceLookupJobCleanup := jobs3.Submit(
		t, "./input/nomad_provider_service_lookup.nomad")
	t.Cleanup(serviceLookupJobCleanup)
	serviceLookupAllocID := serviceLookupJobSubmission.AllocID("nomad_provider_service_lookup")

	mustWaitForTaskFile(t, serviceLookupAllocID, "test", "${NOMAD_TASK_DIR}/services.conf",
		func(out string) error {

			// Ensure the listing (nomadServices) template function has found all
			// services within the default namespace...
			expect := "service default-nomad-provider-service-primary [bar foo]"
			if !strings.Contains(out, expect) {
				return fmt.Errorf("expected %q, got %q", expect, out)
			}
			expect = "service default-nomad-provider-service-secondary [baz buz]"
			if !strings.Contains(out, expect) {
				return fmt.Errorf("expected %q, got %q", expect, out)
			}

			// ... but not the platform namespace.
			expect = "service platform-nomad-provider-service-secondary [baz buz]"
			if strings.Contains(out, expect) {
				return fmt.Errorf("expected %q, got %q", expect, out)
			}
			return nil
		})

	// Ensure the direct service lookup has found the entry we expect.
	expected := fmt.Sprintf("service default-nomad-provider-service-primary [bar foo] dc1 %s", serviceAllocID)

	mustWaitForTaskFile(t, serviceLookupAllocID, "test", "${NOMAD_TASK_DIR}/service.conf",
		func(out string) error {
			if !strings.Contains(out, expected) {
				return fmt.Errorf("expected %q, got %q", expected, out)
			}
			return nil
		})

	// Scale the default namespaced service job in order to change the expected
	// number of entries.
	serviceJobSubmission.Rerun(jobs3.MutateJobSpec(func(spec string) string {
		return strings.Replace(spec, "count = 1", "count = 3", 1)
	}))

	// Pull the allocation ID for the job, we use this to ensure this is found
	// in the rendered template later on. Wrap this in a wait due to the
	// eventual consistency around the service registration process.
	serviceJobAllocs := []map[string]string{}
	var err error
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			serviceJobAllocs, err = e2eutil.AllocsForJob(serviceJobSubmission.JobID(), "default")
			must.NoError(t, err)
			return len(serviceJobAllocs) == 3
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(200*time.Millisecond),
	), must.Sprint("unexpected number of allocs found"))

	// Track the expected entries, including the allocID to make this test
	// actually valuable.
	var expectedEntries []string
	for _, allocs := range serviceJobAllocs {
		e := fmt.Sprintf("service default-nomad-provider-service-primary [bar foo] dc1 %s", allocs["ID"])
		expectedEntries = append(expectedEntries, e)
	}

	// Ensure the direct service lookup has the new entries we expect.
	mustWaitForTaskFile(t, serviceLookupAllocID, "test", "${NOMAD_TASK_DIR}/service.conf",
		func(out string) error {
			for _, entry := range expectedEntries {
				if !strings.Contains(out, entry) {
					return fmt.Errorf("expected %q, got %q", expectedEntries, out)
				}
			}
			return nil
		})
}

// mustWaitForTaskFile is a helper that asserts a file not reachable from alloc
// FS has been rendered and tests its contents
func mustWaitForTaskFile(t *testing.T, allocID, task, path string, testFn func(string) error, testSettings ...must.Setting) {
	t.Helper()

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			out, err := e2eutil.Command("nomad", "alloc", "exec", "-task", task, allocID, "sh", "-c", "cat "+path)
			if err != nil {
				return fmt.Errorf("could not cat file %q from task %q in allocation %q: %v",
					path, task, allocID, err)
			}
			return testFn(out)
		}),
		wait.Gap(time.Millisecond*500),
		wait.Timeout(30*time.Second),
	), testSettings...)
}

// mustWaitTemplateRender is a helper that asserts a file has been rendered and
// tests its contents
func mustWaitTemplateRender(t *testing.T, allocID, path string, testFn func(string) error, timeout time.Duration, testSettings ...must.Setting) {
	t.Helper()

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			out, err := e2eutil.Command("nomad", "alloc", "fs", allocID, path)
			if err != nil {
				return err
			}
			return testFn(out)
		}),
		wait.Gap(time.Millisecond*500),
		wait.Timeout(timeout),
	), testSettings...)
}

// mustWaitForStatusByGroup is similar to e2eutil.WaitForAllocStatus but maps
// specific task group names to statuses without having to deal with specific
// counts
func mustWaitForStatusByGroup(t *testing.T, jobID, ns string,
	expected map[string]string,
	timeout time.Duration, testSettings ...must.Setting) {

	t.Helper()

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			got, err := e2eutil.AllocsForJob(jobID, ns)
			if err != nil {
				return err
			}
			for _, row := range got {
				group := row["Task Group"]
				expectedStatus := expected[group]
				gotStatus := row["Status"]
				if expectedStatus != gotStatus {
					return fmt.Errorf("expected %q to be %q, got %q",
						group, expectedStatus, gotStatus)
				}
			}
			return nil
		}),
		wait.Gap(time.Millisecond*500),
		wait.Timeout(timeout),
	), testSettings...)
}

func cleanupJob(t *testing.T, ns, jobID string) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	t.Helper()
	t.Cleanup(func() {
		e2eutil.StopJob(jobID, "-purge", "-detach", "-namespace", ns)
	})
}

func cleanupGC(t *testing.T) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	t.Helper()
	t.Cleanup(func() {
		_, err := e2eutil.Command("nomad", "system", "gc")
		test.NoError(t, err)
	})
}

func cleanupKey(t *testing.T, key string) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	t.Helper()
	t.Cleanup(func() {
		cc := e2eutil.ConsulClient(t)
		_, err := cc.KV().Delete(key, nil)
		test.NoError(t, err, test.Sprintf("could not clean up consul key: %v", key))
	})
}
