// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"time"

	metrics "github.com/hashicorp/go-metrics/compat"
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

func (v *HostVolume) Create(
	req *cstructs.ClientHostVolumeCreateRequest,
	resp *cstructs.ClientHostVolumeCreateResponse) error {

	defer metrics.MeasureSince([]string{"client", "host_volume", "create"}, time.Now())
	ctx, cancelFn := v.requestContext()
	defer cancelFn()

	cresp, err := v.c.hostVolumeManager.Create(ctx, req)
	if err != nil {
		v.c.logger.Error("failed to create host volume", "name", req.Name, "error", err)
		return err
	}

	resp.CapacityBytes = cresp.CapacityBytes
	resp.HostPath = cresp.HostPath

	v.c.logger.Info("created host volume", "id", req.ID, "path", resp.HostPath)
	return nil
}

func (v *HostVolume) Register(
	req *cstructs.ClientHostVolumeRegisterRequest,
	resp *cstructs.ClientHostVolumeRegisterResponse) error {

	defer metrics.MeasureSince([]string{"client", "host_volume", "register"}, time.Now())
	ctx, cancelFn := v.requestContext()
	defer cancelFn()

	err := v.c.hostVolumeManager.Register(ctx, req)
	if err != nil {
		v.c.logger.Error("failed to register host volume", "name", req.Name, "error", err)
		return err
	}

	v.c.logger.Info("registered host volume", "id", req.ID, "path", req.HostPath)
	return nil
}

func (v *HostVolume) Delete(
	req *cstructs.ClientHostVolumeDeleteRequest,
	resp *cstructs.ClientHostVolumeDeleteResponse) error {
	defer metrics.MeasureSince([]string{"client", "host_volume", "create"}, time.Now())
	ctx, cancelFn := v.requestContext()
	defer cancelFn()

	_, err := v.c.hostVolumeManager.Delete(ctx, req)
	if err != nil {
		v.c.logger.Error("failed to delete host volume", "ID", req.ID, "error", err)
		return err
	}

	v.c.logger.Info("deleted host volume", "id", req.ID, "path", req.HostPath)
	return nil
}

func (v *HostVolume) requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), hostVolumeRequestTimeout)
}
