// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package docker

import docker "github.com/fsouza/go-dockerclient"

// Currently Windows containers don't support host ip in port binding.
func getPortBinding(ip string, port string) docker.PortBinding {
	return docker.PortBinding{HostIP: "", HostPort: port}
}

func tweakCapabilities(basics, adds, drops []string) ([]string, error) {
	return nil, nil
}
