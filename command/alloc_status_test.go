package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocStatusCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &AllocStatusCommand{}
}

func TestAllocStatusCommand_Fails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "foobar"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying allocation") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on missing alloc
	if code := cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No allocation(s) with prefix or id") {
		t.Fatalf("expected not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	if code := cmd.Run([]string{"-address=" + url, "2"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "must contain at least two characters.") {
		t.Fatalf("expected too few characters error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Identifiers with uneven length should produce a query result
	if code := cmd.Run([]string{"-address=" + url, "123"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No allocation(s) with prefix or id") {
		t.Fatalf("expected not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Failed on both -json and -t options are specified
	if code := cmd.Run([]string{"-address=" + url, "-json", "-t", "{{.ID}}"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Both json and template formatting are not allowed") {
		t.Fatalf("expected getting formatter error, got: %s", out)
	}
}

func TestAllocStatusCommand_LifecycleInfo(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		require.NoError(t, err)
	})

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	state := srv.Agent.Server().State()

	a := mock.Alloc()
	a.Metrics = &structs.AllocMetric{}
	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	initTask := tg.Tasks[0].Copy()
	initTask.Name = "init_task"
	initTask.Lifecycle = &structs.TaskLifecycleConfig{
		Hook: "prestart",
	}

	prestartSidecarTask := tg.Tasks[0].Copy()
	prestartSidecarTask.Name = "prestart_sidecar"
	prestartSidecarTask.Lifecycle = &structs.TaskLifecycleConfig{
		Hook:    "prestart",
		Sidecar: true,
	}

	tg.Tasks = append(tg.Tasks, initTask, prestartSidecarTask)
	a.TaskResources["init_task"] = a.TaskResources["web"]
	a.TaskResources["prestart_sidecar"] = a.TaskResources["web"]
	a.TaskStates = map[string]*structs.TaskState{
		"web":              {State: "pending"},
		"init_task":        {State: "running"},
		"prestart_sidecar": {State: "running"},
	}

	require.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	if code := cmd.Run([]string{"-address=" + url, a.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()

	require.Contains(t, out, `Task "init_task" (prestart) is "running"`)
	require.Contains(t, out, `Task "prestart_sidecar" (prestart sidecar) is "running"`)
	require.Contains(t, out, `Task "web" is "pending"`)
}

func TestAllocStatusCommand_Run(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if _, ok := node.Drivers["mock_driver"]; ok &&
				node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}
	// get an alloc id
	allocId1 := ""
	nodeName := ""
	if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
		if len(allocs) > 0 {
			allocId1 = allocs[0].ID
			nodeName = allocs[0].NodeName
		}
	}
	if allocId1 == "" {
		t.Fatal("unable to find an allocation")
	}

	if code := cmd.Run([]string{"-address=" + url, allocId1}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "Created") {
		t.Fatalf("expected to have 'Created' but saw: %s", out)
	}

	if !strings.Contains(out, "Modified") {
		t.Fatalf("expected to have 'Modified' but saw: %s", out)
	}

	nodeNameRegexpStr := fmt.Sprintf(`\nNode Name\s+= %s\n`, regexp.QuoteMeta(nodeName))
	require.Regexp(t, regexp.MustCompile(nodeNameRegexpStr), out)

	ui.OutputWriter.Reset()

	if code := cmd.Run([]string{"-address=" + url, "-verbose", allocId1}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, allocId1) {
		t.Fatal("expected to find alloc id in output")
	}
	if !strings.Contains(out, "Created") {
		t.Fatalf("expected to have 'Created' but saw: %s", out)
	}
	ui.OutputWriter.Reset()

	// Try the query with an even prefix that includes the hyphen
	if code := cmd.Run([]string{"-address=" + url, allocId1[:13]}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "Created") {
		t.Fatalf("expected to have 'Created' but saw: %s", out)
	}
	ui.OutputWriter.Reset()

	if code := cmd.Run([]string{"-address=" + url, "-verbose", allocId1}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, allocId1) {
		t.Fatal("expected to find alloc id in output")
	}
	ui.OutputWriter.Reset()

}

func TestAllocStatusCommand_RescheduleInfo(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	// Test reschedule attempt info
	require := require.New(t)
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	a.Metrics = &structs.AllocMetric{}
	nextAllocId := uuid.Generate()
	a.NextAllocation = nextAllocId
	a.RescheduleTracker = &structs.RescheduleTracker{
		Events: []*structs.RescheduleEvent{
			{
				RescheduleTime: time.Now().Add(-2 * time.Minute).UTC().UnixNano(),
				PrevAllocID:    uuid.Generate(),
				PrevNodeID:     uuid.Generate(),
			},
		},
	}
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	if code := cmd.Run([]string{"-address=" + url, a.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	require.Contains(out, "Replacement Alloc ID")
	require.Regexp(regexp.MustCompile(".*Reschedule Attempts\\s*=\\s*1/2"), out)
}

func TestAllocStatusCommand_ScoreMetrics(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	// Test node metrics
	require := require.New(t)
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	mockNode1 := mock.Node()
	mockNode2 := mock.Node()
	a.Metrics = &structs.AllocMetric{
		ScoreMetaData: []*structs.NodeScoreMeta{
			{
				NodeID: mockNode1.ID,
				Scores: map[string]float64{
					"binpack":       0.77,
					"node-affinity": 0.5,
				},
			},
			{
				NodeID: mockNode2.ID,
				Scores: map[string]float64{
					"binpack":       0.75,
					"node-affinity": 0.33,
				},
			},
		},
	}
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	if code := cmd.Run([]string{"-address=" + url, "-verbose", a.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	require.Contains(out, "Placement Metrics")
	require.Contains(out, mockNode1.ID)
	require.Contains(out, mockNode2.ID)

	// assert we sort headers alphabetically
	require.Contains(out, "binpack  node-affinity")
	require.Contains(out, "final score")
}

func TestAllocStatusCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(a.ID, res[0])
}

func TestAllocStatusCommand_HostVolumes(t *testing.T) {
	t.Parallel()
	// We have to create a tempdir for the host volume even though we're
	// not going to use it b/c the server validates the config on startup
	tmpDir, err := ioutil.TempDir("", "vol0")
	if err != nil {
		t.Fatalf("unable to create tempdir for test: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vol0 := uuid.Generate()
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.Client.HostVolumes = []*structs.ClientHostVolumeConfig{
			{
				Name:     vol0,
				Path:     tmpDir,
				ReadOnly: false,
			},
		}
	})
	defer srv.Shutdown()
	state := srv.Agent.Server().State()

	// Upsert the job and alloc
	node := mock.Node()
	alloc := mock.Alloc()
	alloc.Metrics = &structs.AllocMetric{}
	alloc.NodeID = node.ID
	job := alloc.Job
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		vol0: {
			Name:   vol0,
			Type:   structs.VolumeTypeHost,
			Source: tmpDir,
		},
	}
	job.TaskGroups[0].Tasks[0].VolumeMounts = []*structs.VolumeMount{
		{
			Volume:          vol0,
			Destination:     "/var/www",
			ReadOnly:        true,
			PropagationMode: "private",
		},
	}
	// fakes the placement enough so that we have something to iterate
	// on in 'nomad alloc status'
	alloc.TaskStates = map[string]*structs.TaskState{
		"web": {
			Events: []*structs.TaskEvent{
				structs.NewTaskEvent("test event").SetMessage("test msg"),
			},
		},
	}
	summary := mock.JobSummary(alloc.JobID)
	require.NoError(t, state.UpsertJobSummary(1004, summary))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1005, []*structs.Allocation{alloc}))

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	if code := cmd.Run([]string{"-address=" + url, "-verbose", alloc.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Host Volumes")
	require.Contains(t, out, fmt.Sprintf("%s  true", vol0))
	require.NotContains(t, out, "CSI Volumes")
}

func TestAllocStatusCommand_CSIVolumes(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()
	state := srv.Agent.Server().State()

	// Upsert the node, plugin, and volume
	vol0 := uuid.Generate()
	node := mock.Node()
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1001, node)
	require.NoError(t, err)

	vols := []*structs.CSIVolume{{
		ID:             vol0,
		Namespace:      structs.DefaultNamespace,
		PluginID:       "minnie",
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		Topologies: []*structs.CSITopology{{
			Segments: map[string]string{"foo": "bar"},
		}},
	}}
	err = state.CSIVolumeRegister(1002, vols)
	require.NoError(t, err)

	// Upsert the job and alloc
	alloc := mock.Alloc()
	alloc.Metrics = &structs.AllocMetric{}
	alloc.NodeID = node.ID
	job := alloc.Job
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		vol0: {
			Name:   vol0,
			Type:   structs.VolumeTypeCSI,
			Source: vol0,
		},
	}
	job.TaskGroups[0].Tasks[0].VolumeMounts = []*structs.VolumeMount{
		{
			Volume:          vol0,
			Destination:     "/var/www",
			ReadOnly:        true,
			PropagationMode: "private",
		},
	}
	// if we don't set a task state, there's nothing to iterate on alloc status
	alloc.TaskStates = map[string]*structs.TaskState{
		"web": {
			Events: []*structs.TaskEvent{
				structs.NewTaskEvent("test event").SetMessage("test msg"),
			},
		},
	}
	summary := mock.JobSummary(alloc.JobID)
	require.NoError(t, state.UpsertJobSummary(1004, summary))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1005, []*structs.Allocation{alloc}))

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	if code := cmd.Run([]string{"-address=" + url, "-verbose", alloc.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	require.Contains(t, out, "CSI Volumes")
	require.Contains(t, out, fmt.Sprintf("%s  minnie", vol0))
	require.NotContains(t, out, "Host Volumes")
}
