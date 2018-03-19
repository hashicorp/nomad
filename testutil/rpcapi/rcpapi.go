package rpcapi

import (
	"net/rpc"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/structs"
)

type RPC struct {
	Region    string
	Namespace string
	codec     rpc.ClientCodec
}

func NewRPC(codec rpc.ClientCodec) *RPC {
	return &RPC{
		Region:    "global",
		Namespace: structs.DefaultNamespace,
		codec:     codec,
	}
}

// AllocAll calls Alloc.List + Alloc.GetAllocs to return all allocs.
func (r *RPC) AllocAll() ([]*structs.Allocation, error) {
	listResp, err := r.AllocList()
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(listResp.Allocations))
	for _, a := range listResp.Allocations {
		ids = append(ids, a.ID)
	}

	allocsResp, err := r.AllocGetAllocs(ids)
	if err != nil {
		return nil, err
	}
	return allocsResp.Allocs, nil
}

// Alloc.List RPC
func (r *RPC) AllocList() (*structs.AllocListResponse, error) {
	get := &structs.AllocListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    r.Region,
			Namespace: r.Namespace,
		},
	}

	var resp structs.AllocListResponse
	if err := msgpackrpc.CallWithCodec(r.codec, "Alloc.List", get, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Alloc.GetAllocs RPC
func (r *RPC) AllocGetAllocs(ids []string) (*structs.AllocsGetResponse, error) {
	get := &structs.AllocsGetRequest{
		AllocIDs: ids,
		QueryOptions: structs.QueryOptions{
			Region:    r.Region,
			Namespace: r.Namespace,
		},
	}
	var resp structs.AllocsGetResponse
	if err := msgpackrpc.CallWithCodec(r.codec, "Alloc.GetAllocs", get, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Job.List RPC
func (r *RPC) JobList() (*structs.JobListResponse, error) {
	get := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    r.Region,
			Namespace: r.Namespace,
		},
	}

	var resp structs.JobListResponse
	if err := msgpackrpc.CallWithCodec(r.codec, "Job.List", get, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
