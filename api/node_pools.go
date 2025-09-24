// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

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

// NodePools returns a handle on the node pools endpoints.
func (c *Client) NodePools() *NodePools {
	return &NodePools{client: c}
}

// List is used to list all node pools.
func (n *NodePools) List(q *QueryOptions) ([]*NodePool, *QueryMeta, error) {
	var resp []*NodePool
	qm, err := n.client.query("/v1/node/pools", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// PrefixList is used to list node pools that match a given prefix.
func (n *NodePools) PrefixList(prefix string, q *QueryOptions) ([]*NodePool, *QueryMeta, error) {
	if q == nil {
		q = &QueryOptions{}
	}
	q.Prefix = prefix
	return n.List(q)
}

// Info is used to fetch details of a specific node pool.
func (n *NodePools) Info(name string, q *QueryOptions) (*NodePool, *QueryMeta, error) {
	if name == "" {
		return nil, nil, errors.New("missing node pool name")
	}

	var resp NodePool
	qm, err := n.client.query("/v1/node/pool/"+url.PathEscape(name), &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// Register is used to create or update a node pool.
func (n *NodePools) Register(pool *NodePool, w *WriteOptions) (*WriteMeta, error) {
	if pool == nil {
		return nil, errors.New("missing node pool")
	}
	if pool.Name == "" {
		return nil, errors.New("missing node pool name")
	}

	wm, err := n.client.put("/v1/node/pools", pool, nil, w)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Delete is used to delete a node pool.
func (n *NodePools) Delete(name string, w *WriteOptions) (*WriteMeta, error) {
	if name == "" {
		return nil, errors.New("missing node pool name")
	}

	wm, err := n.client.delete("/v1/node/pool/"+url.PathEscape(name), nil, nil, w)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// ListJobs is used to list all the jobs in a node pool.
func (n *NodePools) ListJobs(poolName string, q *QueryOptions) ([]*JobListStub, *QueryMeta, error) {
	var resp []*JobListStub
	qm, err := n.client.query(
		fmt.Sprintf("/v1/node/pool/%s/jobs", url.PathEscape(poolName)),
		&resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// ListNodes is used to list all the nodes in a node pool.
func (n *NodePools) ListNodes(poolName string, q *QueryOptions) ([]*NodeListStub, *QueryMeta, error) {
	var resp []*NodeListStub
	qm, err := n.client.query(
		fmt.Sprintf("/v1/node/pool/%s/nodes", url.PathEscape(poolName)),
		&resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// NodePool is used to serialize a node pool.
type NodePool struct {
	Name                   string                          `hcl:"name,label"`
	Description            string                          `hcl:"description,optional"`
	Meta                   map[string]string               `hcl:"meta,block"`
	NodeIdentityTTL        time.Duration                   `hcl:"node_identity_ttl,optional"`
	SchedulerConfiguration *NodePoolSchedulerConfiguration `hcl:"scheduler_config,block"`
	CreateIndex            uint64
	ModifyIndex            uint64
}

// MarshalJSON implements the json.Marshaler interface and allows
// NodePool.NodeIdentityTTL to be marshaled correctly.
func (n *NodePool) MarshalJSON() ([]byte, error) {
	type Alias NodePool
	exported := &struct {
		NodeIdentityTTL string
		*Alias
	}{
		NodeIdentityTTL: n.NodeIdentityTTL.String(),
		Alias:           (*Alias)(n),
	}
	if n.NodeIdentityTTL == 0 {
		exported.NodeIdentityTTL = ""
	}
	return json.Marshal(exported)
}

// UnmarshalJSON implements the json.Unmarshaler interface and allows
// NodePool.NodeIdentityTTL to be unmarshalled correctly.
func (n *NodePool) UnmarshalJSON(data []byte) (err error) {
	type Alias NodePool
	aux := &struct {
		NodeIdentityTTL any
		*Alias
	}{
		Alias: (*Alias)(n),
	}

	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.NodeIdentityTTL != nil {
		switch v := aux.NodeIdentityTTL.(type) {
		case string:
			if v != "" {
				if n.NodeIdentityTTL, err = time.ParseDuration(v); err != nil {
					return err
				}
			}
		case float64:
			n.NodeIdentityTTL = time.Duration(v)
		}

	}
	return nil
}

// NodePoolSchedulerConfiguration is used to serialize the scheduler
// configuration of a node pool.
type NodePoolSchedulerConfiguration struct {
	SchedulerAlgorithm            SchedulerAlgorithm `hcl:"scheduler_algorithm,optional"`
	MemoryOversubscriptionEnabled *bool              `hcl:"memory_oversubscription_enabled,optional"`
}
