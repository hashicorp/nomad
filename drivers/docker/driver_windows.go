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

func (d *Driver) validateImageUser(user, taskUser string, privileged bool) error {
	if d.config.WindowsAllowInsecureContainerAdmin {
		return nil
	}

	if (user == "ContainerAdmin" || taskUser == "ContainerAdmin") && !privileged {
		return errors.New(
			"running container as ContainerAdmin with Process Isolation is unsafe; change the container user, set task configuration to privileged or disable windows_allow_insecure_container_admin to disable this check",
		)
	}
	return nil
}
