//+build !windows

package docker

import (
	docker "github.com/fsouza/go-dockerclient"
)

func getPortBinding(ip string, port string) docker.PortBinding {
	return docker.PortBinding{HostIP: ip, HostPort: port}
}
