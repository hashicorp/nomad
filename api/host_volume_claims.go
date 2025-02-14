// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import "net/url"

// TaskGroupHostVolumeClaim associates a task group with a host volume ID. It's
// used for stateful deployments, i.e., volume requests with "sticky" set to
// true.
type TaskGroupHostVolumeClaim struct {
	ID            string `mapstructure:"id" hcl:"id"`
	Namespace     string `mapstructure:"namespace" hcl:"namespace"`
	JobID         string `mapstructure:"job_id" hcl:"job_id"`
	TaskGroupName string `mapstructure:"task_group_name" hcl:"task_group_name"`
	AllocID       string `mapstructure:"alloc_id" hcl:"alloc_id"`
	VolumeID      string `mapstructure:"volume_id" hcl:"volume_id"`
	VolumeName    string `mapstructure:"volume_name" hcl:"volume_name"`

	CreateIndex uint64
	ModifyIndex uint64
}

// TaskGroupHostVolumeClaims is used to access the API.
type TaskGroupHostVolumeClaims struct {
	client *Client
}

// TaskGroupHostVolumeClaims returns a new handle on the API.
func (c *Client) TaskGroupHostVolumeClaims() *TaskGroupHostVolumeClaims {
	return &TaskGroupHostVolumeClaims{client: c}
}

func (tgvc *TaskGroupHostVolumeClaims) List(opts *QueryOptions) ([]*TaskGroupHostVolumeClaim, *QueryMeta, error) {
	var out []*TaskGroupHostVolumeClaim

	qm, err := tgvc.client.query("/v1/volumes/claims", &out, opts)
	if err != nil {
		return nil, qm, err
	}
	return out, qm, nil
}

func (tgvc *TaskGroupHostVolumeClaims) Delete(claimID string, opts *WriteOptions) (*WriteMeta, error) {
	path, err := url.JoinPath("/v1/volumes/claim", url.PathEscape(claimID))
	if err != nil {
		return nil, err
	}
	wm, err := tgvc.client.delete(path, nil, nil, opts)
	return wm, err
}
