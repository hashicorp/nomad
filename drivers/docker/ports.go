// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"strconv"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
)

// publishedPorts is a utility struct to keep track of the port bindings to publish.
// After calling add for each port, the publishedPorts and exposedPorts fields can be
// used in the docker container and host configs
type publishedPorts struct {
	logger         hclog.Logger
	publishedPorts map[docker.Port][]docker.PortBinding
	exposedPorts   map[docker.Port]struct{}
}

func newPublishedPorts(logger hclog.Logger) *publishedPorts {
	return &publishedPorts{
		logger:         logger,
		publishedPorts: map[docker.Port][]docker.PortBinding{},
		exposedPorts:   map[docker.Port]struct{}{},
	}
}

// addMapped adds the port to the structures the Docker API expects for declaring mapped ports
func (p *publishedPorts) addMapped(label, ip string, port int, portMap hclutils.MapStrInt) {
	// By default we will map the allocated port 1:1 to the container
	containerPortInt := port

	// If the user has mapped a port using port_map we'll change it here
	if mapped, ok := portMap[label]; ok {
		containerPortInt = mapped
	}

	p.add(label, ip, port, containerPortInt)
}

// add adds a port binding for the given port mapping
func (p *publishedPorts) add(label, ip string, port, to int) {
	// if to is not set, use the port value per default docker functionality
	if to == 0 {
		to = port
	}

	// two docker port bindings are created for each port for tcp and udp
	cPortTCP := docker.Port(strconv.Itoa(to) + "/tcp")
	cPortUDP := docker.Port(strconv.Itoa(to) + "/udp")
	binding := getPortBinding(ip, strconv.Itoa(port))

	if _, ok := p.publishedPorts[cPortTCP]; !ok {
		// initialize both tcp and udp binding slices since they are always created together
		p.publishedPorts[cPortTCP] = []docker.PortBinding{}
		p.publishedPorts[cPortUDP] = []docker.PortBinding{}
	}

	p.publishedPorts[cPortTCP] = append(p.publishedPorts[cPortTCP], binding)
	p.publishedPorts[cPortUDP] = append(p.publishedPorts[cPortUDP], binding)
	p.logger.Debug("allocated static port", "ip", ip, "port", port, "label", label)

	p.exposedPorts[cPortTCP] = struct{}{}
	p.exposedPorts[cPortUDP] = struct{}{}
	p.logger.Debug("exposed port", "port", port, "label", label)
}
