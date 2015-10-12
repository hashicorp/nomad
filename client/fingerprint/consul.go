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

// ConsulFingerprint is used to fingerprint the architecture
type ConsulFingerprint struct {
	logger *log.Logger
}

// NewConsulFingerprint is used to create an OS fingerprint
func NewConsulFingerprint(logger *log.Logger) Fingerprint {
	f := &ConsulFingerprint{logger: logger}
	return f
}

func (f *ConsulFingerprint) Fingerprint(config *client.Config, node *structs.Node) (bool, error) {
	// Guard against uninitialized Links
	if node.Links == nil {
		node.Links = map[string]string{}
	}

	// Get the Consul config
	consulConfig := consul.DefaultConfig()
	if address := config.Read("consul.address"); address != "" {
		consulConfig.Address = address
	}
	if timeoutRaw := config.Read("consul.timeout"); timeoutRaw != "" {
		var err error
		consulConfig.HttpClient.Timeout, err = time.ParseDuration(timeoutRaw)
		if err != nil {
			return false, fmt.Errorf("Unable to parse consul.timeout: %s", err)
		}
	}

	// Make the client
	consulClient, err := consul.NewClient(consulConfig)
	if err != nil {
		return false, fmt.Errorf("Failed to initialize consul client: %s", err)
	}

	// We'll try to detect consul by making a query to to the agent's self API.
	// If we can't hit this URL consul is probably not running on this machine.
	info, err := consulClient.Agent().Self()
	if err != nil {
		return false, fmt.Errorf("Failed to query consul for agent status: %s", err)
	}

	node.Attributes["consul.server"] = strconv.FormatBool(info["Config"]["Server"].(bool))
	node.Attributes["consul.version"] = info["Config"]["Version"].(string)
	node.Attributes["consul.revision"] = info["Config"]["Revision"].(string)
	node.Attributes["consul.name"] = info["Config"]["NodeName"].(string)
	node.Attributes["consul.datacenter"] = info["Config"]["Datacenter"].(string)

	node.Links["consul"] = fmt.Sprintf("%s.%s",
		node.Attributes["consul.datacenter"],
		node.Attributes["consul.name"])

	return true, nil
}
