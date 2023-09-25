// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func Test_consulHook_prepareConsulClientReq(t *testing.T) {
	logger := testlog.HCLogger(t)

	exampleIdentity := &structs.WorkloadIdentity{
		Name:        "consul-test",
		Audience:    []string{"consul.io"},
		Env:         false,
		File:        false,
		ServiceName: "consul-test",
		TTL:         0,
	}

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")
	task.Identity = exampleIdentity
	task.Identities = []*structs.WorkloadIdentity{exampleIdentity}

	mockClient := &consul.MockConsulClient{}
	mockSigner := widmgr.NewMockWIDSigner([]*structs.WorkloadIdentity{exampleIdentity})

	// sign the mock identity
	sids, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID:      alloc.ID,
			TaskName:     task.Name,
			IdentityName: task.Identities[0].Name,
		},
	})
	must.NoError(t, err)

	swids := map[widmgr.TaskIdentity]*structs.SignedWorkloadIdentity{
		{TaskName: task.Name, IdentityName: task.Identities[0].Name}: sids[0],
	}
	mockWIDManager := widmgr.NewMockWIDMgr(swids)
	authMethod := "nomad-workloads"

	// modify the expiry time so that widmgr renewal loop won't try to renew it
	sids[0].Expiration = time.Now().Add(time.Hour)

	consulHook := newConsulHook(logger, alloc, nil, mockWIDManager, mockClient, nil, authMethod)

	tests := []struct {
		name    string
		task    *structs.Task
		want    map[string]consul.JWTLoginRequest
		wantErr bool
	}{
		{
			"empty task",
			&structs.Task{},
			map[string]consul.JWTLoginRequest{},
			false,
		},
		{
			"task that does not use consul",
			&structs.Task{Services: []*structs.Service{{Provider: "nomad"}}},
			map[string]consul.JWTLoginRequest{},
			false,
		},
		{
			"task with consul identity",
			&structs.Task{
				Name: "web",
				Services: []*structs.Service{
					{
						Provider: structs.ServiceProviderConsul,
						Identity: exampleIdentity,
					},
				},
				Identity: exampleIdentity,
			},
			map[string]consul.JWTLoginRequest{"consul-test": {JWT: sids[0].JWT, AuthMethodName: "nomad-workloads"}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := consulHook.prepareConsulClientReq(tt.task)
			if !tt.wantErr {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
			}
			must.Eq(t, tt.want, got)
		})
	}
}
