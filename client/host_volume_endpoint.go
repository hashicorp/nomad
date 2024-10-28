// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"time"

	metrics "github.com/armon/go-metrics"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type HostVolume struct {
	c *Client
}

func newHostVolumesEndpoint(c *Client) *HostVolume {
	v := &HostVolume{c: c}
	return v
}

var hostVolumeRequestTimeout = time.Minute

func (v *HostVolume) Create(req *cstructs.ClientHostVolumeCreateRequest, resp *cstructs.ClientHostVolumeCreateResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "create"}, time.Now())
	ctx, cancelFn := v.requestContext()
	defer cancelFn()

	cresp, err := v.c.hostVolumeManager.Create(ctx, req)
	if err != nil {
		v.c.logger.Debug("failed to create host volume", "name", req.Name, "error", err)
		return err
	}

	//resp.ID = cresp.ID
	resp.CapacityBytes = cresp.CapacityBytes
	resp.HostPath = cresp.HostPath

	v.c.logger.Debug("created host volume", "id", req.ID, "path", resp.HostPath)
	return nil
}

func (v *HostVolume) Delete(req *cstructs.ClientHostVolumeDeleteRequest, resp *cstructs.ClientHostVolumeDeleteResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "create"}, time.Now())
	_, cancelFn := v.requestContext()
	defer cancelFn()

	// TODO(db): call into Client's host volume manager to delete the volume here

	v.c.logger.Debug("deleted host volume", "id", req.ID, "path", req.HostPath)
	return nil
}

func (v *HostVolume) requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), hostVolumeRequestTimeout)
}
