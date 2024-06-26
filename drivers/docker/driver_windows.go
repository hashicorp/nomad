// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package docker

import (
	"errors"

	docker "github.com/fsouza/go-dockerclient"
)

// Currently Windows containers don't support host ip in port binding.
func getPortBinding(ip string, port string) docker.PortBinding {
	return docker.PortBinding{HostIP: "", HostPort: port}
}

func tweakCapabilities(basics, adds, drops []string) ([]string, error) {
	return nil, nil
}

var containerAdminErrMsg = "running container as ContainerAdmin with is unsafe; change the container user, set task configuration to privileged or disable windows_allow_insecure_container_admin to disable this check"

func (d *Driver) validateImageUser(user, taskUser string, driverConfig *TaskConfig) error {
	// we're only interested in the case where isolation is set to "process"
	// (it's also the default) and when windows_allow_insecure_container_admin
	// is explicitly set to true in the config
	if d.config.WindowsAllowInsecureContainerAdmin || driverConfig.Isolation == "hyper-v" {
		return nil
	}

	if (user == "ContainerAdmin" || taskUser == "ContainerAdmin") && !driverConfig.Privileged {
		return errors.New(containerAdminErrMsg)
	}
	return nil
}
