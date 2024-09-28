// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package docker

import (
	"github.com/docker/go-connections/nat"
)

func getPortBinding(ip string, port string) nat.PortBinding {
	return nat.PortBinding{HostIP: ip, HostPort: port}
}

func validateImageUser(imageUser, taskUser string, taskDriverConfig *TaskConfig, driverConfig *DriverConfig) error {
	return nil
}
