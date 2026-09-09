// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestHTTP_GroupSelectionDeploymentJSON(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, func(config *Config) {
		config.Client.Enabled = false
		config.Server.NumSchedulers = new(1)
		for _, consul := range config.Consuls {
			consul.AutoAdvertise = new(false)
			consul.ServerAutoJoin = new(false)
			consul.ClientAutoJoin = new(false)
		}
	}, func(server *TestAgent) {
		state := server.Agent.server.State()
		job := mock.Job()
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, job))
		deployment := structs.NewDeployment(job, job.Priority, time.Now().UnixNano())
		deadline := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		deployment.GroupSelections = map[string]*structs.DeploymentGroupSelection{
			"runtime": {Count: 1, Slots: map[int]*structs.DeploymentGroupSelectionSlot{0: {TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"}}, RequireProgressBy: deadline},
		}
		must.NoError(t, state.UpsertDeployment(1000, deployment))
		// Use the actual HTTP listeners and API decoder. Calling handler methods
		// directly or using encoding/json.Marshal would bypass Nomad's JSON codec.
		client, err := api.NewClient(&api.Config{Address: server.HTTPAddr()})
		must.NoError(t, err)
		listed, _, err := client.Jobs().Deployments(job.ID, false, nil)
		must.NoError(t, err)
		must.Len(t, 1, listed)
		expected := &api.DeploymentGroupSelection{Count: 1, Slots: map[int]*api.DeploymentGroupSelectionSlot{0: {TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"}}, RequireProgressBy: deadline}
		must.Eq(t, expected, listed[0].GroupSelections["runtime"])
		single, _, err := client.Deployments().Info(deployment.ID, nil)
		must.NoError(t, err)
		must.Eq(t, expected, single.GroupSelections["runtime"])
		all, _, err := client.Deployments().List(nil)
		must.NoError(t, err)
		must.Len(t, 1, all)
		must.Eq(t, expected, all[0].GroupSelections["runtime"])
		response, err := http.Get(server.HTTPAddr() + "/v1/job/" + job.ID + "/deployments?pretty=true")
		must.NoError(t, err)
		body, readErr := io.ReadAll(response.Body)
		must.NoError(t, response.Body.Close())
		must.NoError(t, readErr)
		must.Eq(t, http.StatusOK, response.StatusCode)
		must.True(t, json.Valid(body))
		var pretty []*api.Deployment
		must.NoError(t, json.Unmarshal(body, &pretty))
		must.Len(t, 1, pretty)
		must.Eq(t, expected, pretty[0].GroupSelections["runtime"])
	})
}
