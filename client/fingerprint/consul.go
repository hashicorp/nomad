package fingerprint

import (
	"fmt"
	"log"
	"strconv"
	"time"

	consul "github.com/hashicorp/consul/api"

	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	consulAvailable   = "available"
	consulUnavailable = "unavailable"
)

// ConsulFingerprint is used to fingerprint for Consul
type ConsulFingerprint struct {
	logger    *log.Logger
	client    *consul.Client
	lastState string
}

// NewConsulFingerprint is used to create a Consul fingerprint
func NewConsulFingerprint(logger *log.Logger) Fingerprint {
	return &ConsulFingerprint{logger: logger, lastState: consulUnavailable}
}

func (f *ConsulFingerprint) Fingerprint(config *client.Config, node *structs.Node) (bool, error) {
	// Guard against uninitialized Links
	if node.Links == nil {
		node.Links = map[string]string{}
	}

	// Only create the client once to avoid creating too many connections to
	// Consul.
	if f.client == nil {
		consulConfig, err := config.ConsulConfig.ApiConfig()
		if err != nil {
			return false, fmt.Errorf("Failed to initialize the Consul client config: %v", err)
		}

		f.client, err = consul.NewClient(consulConfig)
		if err != nil {
			return false, fmt.Errorf("Failed to initialize consul client: %s", err)
		}
	}

	// We'll try to detect consul by making a query to to the agent's self API.
	// If we can't hit this URL consul is probably not running on this machine.
	info, err := f.client.Agent().Self()
	if err != nil {
		// Clear any attributes set by a previous fingerprint.
		f.clearConsulAttributes(node)

		// Print a message indicating that the Consul Agent is not available
		// anymore
		if f.lastState == consulAvailable {
			f.logger.Printf("[INFO] fingerprint.consul: consul agent is unavailable")
		}
		f.lastState = consulUnavailable
		return false, nil
	}

	if s, ok := info["Config"]["Server"].(bool); ok {
		node.Attributes["consul.server"] = strconv.FormatBool(s)
	} else {
		f.logger.Printf("[WARN] fingerprint.consul: unable to fingerprint consul.server")
	}
	if v, ok := info["Config"]["Version"].(string); ok {
		node.Attributes["consul.version"] = v
	} else {
		f.logger.Printf("[WARN] fingerprint.consul: unable to fingerprint consul.version")
	}
	if r, ok := info["Config"]["Revision"].(string); ok {
		node.Attributes["consul.revision"] = r
	} else {
		f.logger.Printf("[WARN] fingerprint.consul: unable to fingerprint consul.revision")
	}
	if n, ok := info["Config"]["NodeName"].(string); ok {
		node.Attributes["unique.consul.name"] = n
	} else {
		f.logger.Printf("[WARN] fingerprint.consul: unable to fingerprint unique.consul.name")
	}
	if d, ok := info["Config"]["Datacenter"].(string); ok {
		node.Attributes["consul.datacenter"] = d
	} else {
		f.logger.Printf("[WARN] fingerprint.consul: unable to fingerprint consul.datacenter")
	}

	if node.Attributes["consul.datacenter"] != "" || node.Attributes["unique.consul.name"] != "" {
		node.Links["consul"] = fmt.Sprintf("%s.%s",
			node.Attributes["consul.datacenter"],
			node.Attributes["unique.consul.name"])
	} else {
		f.logger.Printf("[WARN] fingerprint.consul: malformed Consul response prevented linking")
	}

	// If the Consul Agent was previously unavailable print a message to
	// indicate the Agent is available now
	if f.lastState == consulUnavailable {
		f.logger.Printf("[INFO] fingerprint.consul: consul agent is available")
	}
	f.lastState = consulAvailable
	return true, nil
}

// clearConsulAttributes removes consul attributes and links from the passed
// Node.
func (f *ConsulFingerprint) clearConsulAttributes(n *structs.Node) {
	delete(n.Attributes, "consul.server")
	delete(n.Attributes, "consul.version")
	delete(n.Attributes, "consul.revision")
	delete(n.Attributes, "unique.consul.name")
	delete(n.Attributes, "consul.datacenter")
	delete(n.Links, "consul")
}

func (f *ConsulFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}
