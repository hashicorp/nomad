package docker

import (
	"strconv"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
)

type publishedPorts struct {
	logger         hclog.Logger
	portMap        hclutils.MapStrInt
	publishedPorts map[docker.Port][]docker.PortBinding
	exposedPorts   map[docker.Port]struct{}
}

func newPublishedPorts(logger hclog.Logger, portMap hclutils.MapStrInt) *publishedPorts {
	return &publishedPorts{
		logger:         logger,
		portMap:        portMap,
		publishedPorts: map[docker.Port][]docker.PortBinding{},
		exposedPorts:   map[docker.Port]struct{}{},
	}
}

// adds the port to the structures the Docker API expects for declaring mapped ports
// if exclusive is set, the port is only added if it is found in the port map config
func (p *publishedPorts) add(label, ip string, port int, exclusive bool) {
	// By default we will map the allocated port 1:1 to the container
	containerPortInt := port

	// If the user has mapped a port using port_map we'll change it here
	if mapped, ok := p.portMap[label]; ok {
		containerPortInt = mapped
	} else if exclusive {
		return
	}

	hostPortStr := strconv.Itoa(port)
	containerPort := docker.Port(strconv.Itoa(containerPortInt))

	p.publishedPorts[containerPort+"/tcp"] = getPortBinding(ip, hostPortStr)
	p.publishedPorts[containerPort+"/udp"] = getPortBinding(ip, hostPortStr)
	p.logger.Debug("allocated static port", "ip", ip, "port", port)

	p.exposedPorts[containerPort+"/tcp"] = struct{}{}
	p.exposedPorts[containerPort+"/udp"] = struct{}{}
	p.logger.Debug("exposed port", "port", port)
}
