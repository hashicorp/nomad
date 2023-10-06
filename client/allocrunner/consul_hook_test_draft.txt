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

	exampleService := &structs.Service{
		TaskName: "consul-test",
		Identity: exampleIdentity,
	}

	alloc := mock.Alloc()
	mockSigner := widmgr.NewMockWIDSigner([]*structs.WorkloadIdentity{exampleIdentity})

	// sign the example identity
	sids, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID:      alloc.ID,
			TaskName:     exampleService.TaskName,
			IdentityName: exampleIdentity.Name,
		},
	})
	must.NoError(t, err)

	swids := map[widmgr.TaskIdentity]*structs.SignedWorkloadIdentity{
		{TaskName: exampleService.TaskName, IdentityName: exampleIdentity.Name}: sids[0],
	}
	mockWIDManager := widmgr.NewMockWIDMgr(swids)

	// modify the expiry time so that widmgr renewal loop won't try to renew it
	sids[0].Expiration = time.Now().Add(time.Hour)

	consulHook := newConsulHook(logger, alloc, nil, mockWIDManager, nil, nil)

	tests := []struct {
		name    string
		task    *structs.Service
		want    map[string]consul.JWTLoginRequest
		wantErr bool
	}{
		{
			"empty service",
			&structs.Service{},
			map[string]consul.JWTLoginRequest{},
			false,
		},
		{
			"service that does not use consul",
			&structs.Service{Provider: "nomad"},
			map[string]consul.JWTLoginRequest{},
			false,
		},
		{
			"service with consul identity",
			&structs.Service{
				Provider: structs.ServiceProviderConsul,
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
