// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

const (
	// NodePoolAll is the node pool that always includes all nodes.
	NodePoolAll = "all"

	// NodePoolDefault is the default node pool.
	NodePoolDefault = "default"
)

// NodePools is used to access node pools endpoints.
type NodePools struct {
	client *Client
}

func (c *Client) NodePools() *NodePools {
	return &NodePools{client: c}
}

func (n *NodePools) List(q *QueryOptions) ([]*NodePool, *QueryMeta, error) {
	var resp []*NodePool
	qm, err := n.client.query("/v1/node/pools", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

func (n *NodePools) PrefixList(prefix string, q *QueryOptions) ([]*NodePool, *QueryMeta, error) {
	if q == nil {
		q = &QueryOptions{}
	}
	q.Prefix = prefix
	return n.List(q)
}

func (n *NodePools) Info(name string, q *QueryOptions) (*NodePool, *QueryMeta, error) {
	var resp NodePool
	qm, err := n.client.query("/v1/node/pool/"+name, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

func (n *NodePools) Register(pool *NodePool, w *WriteOptions) (*WriteMeta, error) {
	wm, err := n.client.put("/v1/node/pools", pool, nil, w)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

func (n *NodePools) Delete(name string, w *WriteOptions) (*WriteMeta, error) {
	wm, err := n.client.delete("/v1/node/pool/"+name, nil, nil, w)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

type NodePool struct {
	Name                   string                          `hcl:"name,label"`
	Description            string                          `hcl:"description,optional"`
	Meta                   map[string]string               `hcl:"meta,block"`
	SchedulerConfiguration *NodePoolSchedulerConfiguration `hcl:"scheduler_configuration,block"`
	CreateIndex            uint64
	ModifyIndex            uint64
}

type NodePoolSchedulerConfiguration struct {
	SchedulerAlgorithm SchedulerAlgorithm `hcl:"scheduler_algorithm,optional"`
}
