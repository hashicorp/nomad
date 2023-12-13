// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux
// +build !linux

package allocrunner

import (
	hclog "github.com/hashicorp/go-hclog"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// noopNetworkManager implements the drivers.DriverNetoworkManager interface to
// provide a no-op manager for systems that don't support network isolation.
type noopNetworkManager struct{}

func (*noopNetworkManager) CreateNetwork(_ string, _ *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
	return nil, false, nil
}

func (*noopNetworkManager) DestroyNetwork(_ string, _ *drivers.NetworkIsolationSpec) error {
	return nil
}

// TODO: Support windows shared networking
func newNetworkManager(alloc *structs.Allocation, driverManager drivermanager.Manager) (nm drivers.DriverNetworkManager, err error) {
	return &noopNetworkManager{}, nil
}

func newNetworkConfigurator(log hclog.Logger, alloc *structs.Allocation, config *clientconfig.Config) (NetworkConfigurator, error) {
	return &hostNetworkConfigurator{}, nil
}
