// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestPluginStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &PluginStatusCommand{}
}

func TestPluginStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &PluginStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Test an unsupported plugin type.
	code = cmd.Run([]string{"-type=not-a-plugin"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Unsupported plugin type: not-a-plugin")
	ui.ErrorWriter.Reset()
}

func TestPluginStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &PluginStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a plugin
	id := "long-plugin-id"
	s := srv.Agent.Server().State()
	cleanup := state.CreateTestCSIPlugin(s, id)
	defer cleanup()
	ws := memdb.NewWatchSet()
	plug, err := s.CSIPluginByID(ws, id)
	must.NoError(t, err)

	prefix := plug.ID[:len(plug.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, plug.ID, res[0])
}

func TestPluginStatusCommand_formatControllerCaps(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name             string
		inputControllers map[string]*api.CSIInfo
		expectedOutput   string
	}{
		{
			name:             "no controllers",
			inputControllers: map[string]*api.CSIInfo{},
			expectedOutput:   "",
		},
		{
			name: "single controller with all capabilities",
			inputControllers: map[string]*api.CSIInfo{
				"node-1": {
					ControllerInfo: &api.CSIControllerInfo{
						SupportsCreateDelete:             true,
						SupportsAttachDetach:             true,
						SupportsListVolumes:              true,
						SupportsGetCapacity:              true,
						SupportsCreateDeleteSnapshot:     true,
						SupportsListSnapshots:            true,
						SupportsClone:                    true,
						SupportsReadOnlyAttach:           true,
						SupportsExpand:                   true,
						SupportsListVolumesAttachedNodes: true,
						SupportsCondition:                true,
						SupportsGet:                      true,
					},
				},
			},
			expectedOutput: "  ATTACH_READONLY\n  CLONE_VOLUME\n  CONTROLLER_ATTACH_DETACH\n  CREATE_DELETE_SNAPSHOT\n  CREATE_DELETE_VOLUME\n  EXPAND_VOLUME\n  GET_CAPACITY\n  GET_VOLUME\n  LIST_SNAPSHOTS\n  LIST_VOLUMES\n  LIST_VOLUMES_PUBLISHED_NODES\n  VOLUME_CONDITION",
		},
		{
			name: "single controller with one capability",
			inputControllers: map[string]*api.CSIInfo{
				"node-1": {
					ControllerInfo: &api.CSIControllerInfo{
						SupportsCreateDelete:             false,
						SupportsAttachDetach:             true,
						SupportsListVolumes:              false,
						SupportsGetCapacity:              false,
						SupportsCreateDeleteSnapshot:     false,
						SupportsListSnapshots:            false,
						SupportsClone:                    false,
						SupportsReadOnlyAttach:           false,
						SupportsExpand:                   false,
						SupportsListVolumesAttachedNodes: false,
						SupportsCondition:                false,
						SupportsGet:                      false,
					},
				},
			},
			expectedOutput: "  CONTROLLER_ATTACH_DETACH",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &PluginStatusCommand{}
			must.Eq(t, tc.expectedOutput, cmd.formatControllerCaps(tc.inputControllers))
		})
	}
}

func TestPluginStatusCommand_formatNodeCaps(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name           string
		inputNodes     map[string]*api.CSIInfo
		expectedOutput string
	}{
		{
			name:           "no nodes",
			inputNodes:     map[string]*api.CSIInfo{},
			expectedOutput: "",
		},
		{
			name: "single node with all capabilities and topologies",
			inputNodes: map[string]*api.CSIInfo{
				"node-1": {
					RequiresTopologies: true,
					NodeInfo: &api.CSINodeInfo{
						RequiresNodeStageVolume: true,
						SupportsStats:           true,
						SupportsExpand:          true,
						SupportsCondition:       true,
					},
				},
			},
			expectedOutput: "  EXPAND_VOLUME\n  GET_VOLUME_STATS\n  STAGE_UNSTAGE_VOLUME\n  VOLUME_ACCESSIBILITY_CONSTRAINTS\n  VOLUME_CONDITION",
		},
		{
			name: "single node with single capability and no topologies",
			inputNodes: map[string]*api.CSIInfo{
				"node-1": {
					RequiresTopologies: false,
					NodeInfo: &api.CSINodeInfo{
						RequiresNodeStageVolume: true,
						SupportsStats:           false,
						SupportsExpand:          false,
						SupportsCondition:       false,
					},
				},
			},
			expectedOutput: "  STAGE_UNSTAGE_VOLUME",
		},
		{
			name: "single node with subset of capabilities",
			inputNodes: map[string]*api.CSIInfo{
				"node-1": {
					RequiresTopologies: false,
					NodeInfo: &api.CSINodeInfo{
						RequiresNodeStageVolume: false,
						SupportsStats:           true,
						SupportsExpand:          false,
						SupportsCondition:       true,
					},
				},
			},
			expectedOutput: "  GET_VOLUME_STATS\n  VOLUME_CONDITION",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &PluginStatusCommand{}
			must.Eq(t, tc.expectedOutput, cmd.formatNodeCaps(tc.inputNodes))
		})
	}
}
