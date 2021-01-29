package qemu

import (
	"fmt"
	"strconv"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
)

// PortForwardRule defines a single set of configuration to create a QEMU port
// forwarding rule
type portForwardRule struct {
	Protocol  string
	HostIP    string
	HostPort  int
	GuestIP   string
	GuestPort int
}

type publishedPorts struct {
	logger       hclog.Logger
	forwardRules map[string]portForwardRule
}

func newPublishedPorts(logger hclog.Logger) *publishedPorts {
	return &publishedPorts{
		logger:       logger,
		forwardRules: make(map[string]portForwardRule),
	}
}

func getSupportedProtocols() []string {
	return []string{"udp", "tcp"}
}

//adds the port to the structures the Docker API expects for declaring mapped ports
func (p *publishedPorts) addMapped(label, ip string, port int, portMap hclutils.MapStrInt) {
	// By default we will map the allocated port 1:1 to the container
	containerPortInt := port

	// If the user has mapped a port using port_map we'll change it here
	if mapped, ok := portMap[label]; ok {
		containerPortInt = mapped
	}

	p.add(label, ip, port, containerPortInt)
}

func (p *publishedPorts) add(label, ip string, port, to int) {
	if to == 0 {
		to = port
	}
	destinationPort := strconv.Itoa(to)
	for _, proto := range getSupportedProtocols() {

		p.forwardRules[destinationPort+"/"+proto] = portForwardRule{
			Protocol:  proto,
			HostIP:    "",
			HostPort:  port,
			GuestIP:   "",
			GuestPort: to,
		}
	}
	p.logger.Debug("allocated static port", "ip", ip, "port", port)
}

func (p *publishedPorts) toString() string {
	return strings.Join(p.toStringArray(), ",")
}

func (p *publishedPorts) toStringArray() []string {
	ruleStrings := []string{}
	for _, rule := range p.forwardRules {
		ruleStrings = append(ruleStrings, fmt.Sprintf("hostfwd=%s:%s:%d-%s:%d",
			rule.Protocol,
			rule.HostIP,
			rule.HostPort,
			rule.GuestIP,
			rule.GuestPort))
	}
	return ruleStrings
}
