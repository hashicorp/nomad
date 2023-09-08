// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestAllocStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &AllocStatusCommand{}
}

func TestAllocStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))

	ui.ErrorWriter.Reset()

	// Fails on connection failure
	code = cmd.Run([]string{"-address=nope", "foobar"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Error querying allocation")

	ui.ErrorWriter.Reset()

	// Fails on missing alloc
	code = cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No allocation(s) with prefix or id")

	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	code = cmd.Run([]string{"-address=" + url, "2"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "must contain at least two characters.")

	ui.ErrorWriter.Reset()

	// Identifiers with uneven length should produce a query result
	code = cmd.Run([]string{"-address=" + url, "123"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No allocation(s) with prefix or id")

	ui.ErrorWriter.Reset()

	// Failed on both -json and -t options are specified
	code = cmd.Run([]string{"-address=" + url, "-json", "-t", "{{.ID}}"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Both json and template formatting are not allowed")
}

func TestAllocStatusCommand_LifecycleInfo(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

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

	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	code := cmd.Run([]string{"-address=" + url, a.ID})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, `Task "init_task" (prestart) is "running"`)
	must.StrContains(t, out, `Task "prestart_sidecar" (prestart sidecar) is "running"`)
	must.StrContains(t, out, `Task "web" is "pending"`)
}

func TestAllocStatusCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	must.NoError(t, err)

	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	// get an alloc id
	allocID := ""
	nodeName := ""
	if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
		if len(allocs) > 0 {
			allocID = allocs[0].ID
			nodeName = allocs[0].NodeName
		}
	}
	must.NotEq(t, "", allocID)

	code = cmd.Run([]string{"-address=" + url, allocID})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Created")
	must.StrContains(t, out, "Modified")

	nodeNameRegexpStr := fmt.Sprintf(`\nNode Name\s+= %s\n`, regexp.QuoteMeta(nodeName))
	must.RegexMatch(t, regexp.MustCompile(nodeNameRegexpStr), out)

	ui.OutputWriter.Reset()

	code = cmd.Run([]string{"-address=" + url, "-verbose", allocID})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrContains(t, out, allocID)
	must.StrContains(t, out, "Created")

	ui.OutputWriter.Reset()

	// Try the query with an even prefix that includes the hyphen
	code = cmd.Run([]string{"-address=" + url, allocID[:13]})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrContains(t, out, "Created")
	ui.OutputWriter.Reset()

	code = cmd.Run([]string{"-address=" + url, "-verbose", allocID})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrContains(t, out, allocID)

	// make sure nsd checks status output is elided if none exist
	must.StrNotContains(t, out, `Nomad Service Checks:`)
}

func TestAllocStatusCommand_RescheduleInfo(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	// Test reschedule attempt info
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
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	if code := cmd.Run([]string{"-address=" + url, a.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Replacement Alloc ID")
	must.RegexMatch(t, regexp.MustCompile(".*Reschedule Attempts\\s*=\\s*1/2"), out)
}

func TestAllocStatusCommand_ScoreMetrics(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}

	// Test node metrics
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
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	code := cmd.Run([]string{"-address=" + url, "-verbose", a.ID})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Placement Metrics")
	must.StrContains(t, out, mockNode1.ID)
	must.StrContains(t, out, mockNode2.ID)

	// assert we sort headers alphabetically
	must.StrContains(t, out, "binpack  node-affinity")
	must.StrContains(t, out, "final score")
}

func TestAllocStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, a.ID, res[0])
}

func TestAllocStatusCommand_HostVolumes(t *testing.T) {
	ci.Parallel(t)
	// We have to create a tempdir for the host volume even though we're
	// not going to use it b/c the server validates the config on startup
	tmpDir := t.TempDir()

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
	must.NoError(t, state.UpsertJobSummary(1004, summary))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1005, []*structs.Allocation{alloc}))

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	code := cmd.Run([]string{"-address=" + url, "-verbose", alloc.ID})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Host Volumes")
	must.StrContains(t, out, fmt.Sprintf("%s  true", vol0))
	must.StrNotContains(t, out, "CSI Volumes")
}

func TestAllocStatusCommand_CSIVolumes(t *testing.T) {
	ci.Parallel(t)
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
	must.NoError(t, err)

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
	err = state.UpsertCSIVolume(1002, vols)
	must.NoError(t, err)

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
	must.NoError(t, state.UpsertJobSummary(1004, summary))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1005, []*structs.Allocation{alloc}))

	ui := cli.NewMockUi()
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}
	if code := cmd.Run([]string{"-address=" + url, "-verbose", alloc.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "CSI Volumes")
	must.StrContains(t, out, fmt.Sprintf("%s  minnie", vol0))
	must.StrNotContains(t, out, "Host Volumes")
}

func TestAllocStatusCommand_NSD_Checks(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// wait for nodes
	waitForNodes(t, client)

	jobID := "job1_checks"
	job1 := testNomadServiceJob(jobID)

	resp, _, err := client.Jobs().Register(job1, nil)
	must.NoError(t, err)

	// wait for registration success
	ui := cli.NewMockUi()
	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	// Get an alloc id
	allocID := getAllocFromJob(t, client, jobID)

	// wait for the check to be marked failure
	waitForCheckStatus(t, client, allocID, "failure")

	// Run command
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}
	code = cmd.Run([]string{"-address=" + url, allocID})
	must.Zero(t, code)

	// check output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, `Nomad Service Checks:`)
	must.RegexMatch(t, regexp.MustCompile(`Service\s+Task\s+Name\s+Mode\s+Status`), out)
	must.RegexMatch(t, regexp.MustCompile(`service1\s+\(group\)\s+check1\s+healthiness\s+(pending|failure)`), out)
}
