// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package widmgr

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type RPCer interface {
	RPC(method string, args any, reply any) error
}

// Config wraps the configuration parameters the workload identity manager
// needs.
type Config struct {
	// NodeSecret is the node's secret token
	NodeSecret string

	// Region of the node
	Region string

	RPC RPCer
}

// WIDMgr fetches and validates workload identities.
type WIDMgr struct {
	nodeSecret string
	region     string
	rpc        RPCer
}

// New workload identity manager.
func New(c Config) *WIDMgr {
	return &WIDMgr{
		nodeSecret: c.NodeSecret,
		region:     c.Region,
		rpc:        c.RPC,
	}
}

// SignIdentities wraps the Alloc.SignIdentities RPC and retrieves signed
// workload identities. The minIndex should be set to the lowest allocation
// CreateIndex to ensure that the server handling the request isn't so stale
// that it doesn't know the allocation exist (and therefore rejects the signing
// requests).
//
// Since a single rejection causes an error to be returned, SignIdentities
// should currently only be used when requesting signed identities for a single
// allocation.
func (m *WIDMgr) SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error) {
	args := structs.AllocIdentitiesRequest{
		Identities: req,
		QueryOptions: structs.QueryOptions{
			Region:        m.region,
			MinQueryIndex: minIndex - 1,
			AllowStale:    true,
			AuthToken:     m.nodeSecret,
		},
	}
	reply := structs.AllocIdentitiesResponse{}
	if err := m.rpc.RPC("Alloc.SignIdentities", &args, &reply); err != nil {
		return nil, err
	}

	if n := len(reply.Rejections); n == 1 {
		return nil, fmt.Errorf("%d/%d signing request was rejected", n, len(req))
	} else if n > 1 {
		return nil, fmt.Errorf("%d/%d signing requests were rejected", n, len(req))
	}

	if len(reply.SignedIdentities) == 0 {
		return nil, fmt.Errorf("empty signed identity response")
	}

	if exp, act := len(reply.SignedIdentities), len(req); exp != act {
		return nil, fmt.Errorf("expected %d signed identities but received %d", exp, act)
	}

	return reply.SignedIdentities, nil
}
