package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"strconv"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	require2 "github.com/stretchr/testify/require"
)

func TestPlanCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &JobRunCommand{}
}

func TestPlanCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &JobPlanCommand{Meta: Meta{Ui: ui}}

	// Create a server
	s := testutil.NewTestServer(t, nil)
	defer s.Stop()
	os.Setenv("NOMAD_ADDR", fmt.Sprintf("http://%s", s.HTTPAddr))

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 255 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails when specified file does not exist
	if code := cmd.Run([]string{"/unicorns/leprechauns"}); code != 255 {
		t.Fatalf("expect exit 255, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error getting job struct") {
		t.Fatalf("expect getting job struct error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on invalid HCL
	fh1, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh1.Name())
	if _, err := fh1.WriteString("nope"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{fh1.Name()}); code != 255 {
		t.Fatalf("expect exit 255, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error getting job struct") {
		t.Fatalf("expect parsing error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on invalid job spec
	fh2, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh2.Name())
	if _, err := fh2.WriteString(`job "job1" {}`); err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{fh2.Name()}); code != 255 {
		t.Fatalf("expect exit 255, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error during plan") {
		t.Fatalf("expect validation error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure (requires a valid job)
	fh3, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh3.Name())
	_, err = fh3.WriteString(`
job "job1" {
	type = "service"
	datacenters = [ "dc1" ]
	group "group1" {
		count = 1
		task "task1" {
			driver = "exec"
			resources = {
				cpu = 1000
				memory = 512
			}
		}
	}
}`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{"-address=nope", fh3.Name()}); code != 255 {
		t.Fatalf("expected exit code 255, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error during plan") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}

func TestPlanCommand_From_STDIN(t *testing.T) {
	t.Parallel()
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := new(cli.MockUi)
	cmd := &JobPlanCommand{
		Meta:      Meta{Ui: ui},
		JobGetter: JobGetter{testStdin: stdinR},
	}

	go func() {
		stdinW.WriteString(`
job "job1" {
  type = "service"
  datacenters = [ "dc1" ]
  group "group1" {
                count = 1
                task "task1" {
                        driver = "exec"
                        resources = {
                                cpu = 1000
                                memory = 512
                        }
                }
        }
}`)
		stdinW.Close()
	}()

	args := []string{"-"}
	if code := cmd.Run(args); code != 255 {
		t.Fatalf("expected exit code 255, got %d: %q", code, ui.ErrorWriter.String())
	}

	if out := ui.ErrorWriter.String(); !strings.Contains(out, "connection refused") {
		t.Fatalf("expected connection refused error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestPlanCommand_From_URL(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &JobPlanCommand{
		Meta: Meta{Ui: ui},
	}

	args := []string{"https://example.com/foo/bar"}
	if code := cmd.Run(args); code != 255 {
		t.Fatalf("expected exit code 255, got %d: %q", code, ui.ErrorWriter.String())
	}

	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error getting jobfile") {
		t.Fatalf("expected error getting jobfile, got: %s", out)
	}
}

func TestPlanCommad_Preemptions(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &JobPlanCommand{Meta: Meta{Ui: ui}}
	require := require2.New(t)

	// Only one preempted alloc
	resp1 := &api.JobPlanResponse{
		Annotations: &api.PlanAnnotations{
			PreemptedAllocs: []*api.AllocationListStub{
				{
					ID:        "alloc1",
					JobID:     "jobID1",
					TaskGroup: "meta",
					JobType:   "batch",
					Namespace: "test",
				},
			},
		},
	}
	cmd.addPreemptions(resp1)
	out := ui.OutputWriter.String()
	require.Contains(out, "Alloc ID")
	require.Contains(out, "alloc1")

	// Less than 10 unique job ids
	var preemptedAllocs []*api.AllocationListStub
	for i := 0; i < 12; i++ {
		job_id := "job" + strconv.Itoa(i%4)
		alloc := &api.AllocationListStub{
			ID:        "alloc",
			JobID:     job_id,
			TaskGroup: "meta",
			JobType:   "batch",
			Namespace: "test",
		}
		preemptedAllocs = append(preemptedAllocs, alloc)
	}

	resp2 := &api.JobPlanResponse{
		Annotations: &api.PlanAnnotations{
			PreemptedAllocs: preemptedAllocs,
		},
	}
	ui.OutputWriter.Reset()
	cmd.addPreemptions(resp2)
	out = ui.OutputWriter.String()
	require.Contains(out, "Job ID")
	require.Contains(out, "Namespace")

	// More than 10 unique job IDs
	preemptedAllocs = make([]*api.AllocationListStub, 0)
	job_type := "batch"
	for i := 0; i < 20; i++ {
		job_id := "job" + strconv.Itoa(i)
		if i%2 == 0 {
			job_type = "service"
		} else {
			job_type = "batch"
		}
		alloc := &api.AllocationListStub{
			ID:        "alloc",
			JobID:     job_id,
			TaskGroup: "meta",
			JobType:   job_type,
			Namespace: "test",
		}
		preemptedAllocs = append(preemptedAllocs, alloc)
	}

	resp3 := &api.JobPlanResponse{
		Annotations: &api.PlanAnnotations{
			PreemptedAllocs: preemptedAllocs,
		},
	}
	ui.OutputWriter.Reset()
	cmd.addPreemptions(resp3)
	out = ui.OutputWriter.String()
	require.Contains(out, "Job Type")
	require.Contains(out, "batch")
	require.Contains(out, "service")
}
