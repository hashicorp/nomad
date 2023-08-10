// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	// HookNameDevices is the name of the devices hook
	HookNameDevices = "devices"
)

// deviceHook is used to retrieve device mounting information.
type deviceHook struct {
	logger log.Logger
	dm     devicemanager.Manager
}

func newDeviceHook(dm devicemanager.Manager, logger log.Logger) *deviceHook {
	h := &deviceHook{
		dm: dm,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*deviceHook) Name() string {
	return HookNameDevices
}

func (h *deviceHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	//TODO Can the nil check be removed once the TODO in NewTaskRunner
	//     where this is set is addressed?
	if req.TaskResources == nil || len(req.TaskResources.Devices) == 0 {
		resp.Done = true
		return nil
	}

	// Capture the responses
	var reservations []*device.ContainerReservation
	for _, req := range req.TaskResources.Devices {
		// Ask the device manager for the reservation information
		res, err := h.dm.Reserve(req)
		if err != nil {
			return fmt.Errorf("failed to reserve device %s: %v", req.ID(), err)
		}

		reservations = append(reservations, res)
	}

	// Build the response
	for _, res := range reservations {
		for k, v := range res.Envs {
			if resp.Env == nil {
				resp.Env = make(map[string]string)
			}

			resp.Env[k] = v
		}

		for _, m := range res.Mounts {
			resp.Mounts = append(resp.Mounts, convertMount(m))
		}

		for _, d := range res.Devices {
			resp.Devices = append(resp.Devices, convertDevice(d))
		}
	}

	resp.Done = true
	return nil
}

func convertMount(in *device.Mount) *drivers.MountConfig {
	return &drivers.MountConfig{
		TaskPath: in.TaskPath,
		HostPath: in.HostPath,
		Readonly: in.ReadOnly,
	}
}

func convertDevice(in *device.DeviceSpec) *drivers.DeviceConfig {
	return &drivers.DeviceConfig{
		TaskPath:    in.TaskPath,
		HostPath:    in.HostPath,
		Permissions: in.CgroupPerms,
	}
}
