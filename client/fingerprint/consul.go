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

	// We'll try to automatically defect consul by making a query to
	//	http://127.0.0.1:8500/v1/agent/self
	// If we can't hit this URL consul is probably not running on this machine.
	// Also since this is local and should be basically instant we'll set a very
	// short timeout so we don't block Nomad from starting up for too long.

	// TODO make this configurable
	timeout, _ := time.ParseDuration("100ms")

	consulConfig := consul.DefaultConfig()
	consulConfig.Address = "172.16.59.133:8500"
	consulConfig.HttpClient.Timeout = timeout

	consulClient, err := consul.NewClient(consulConfig)
	if err != nil {
		return false, fmt.Errorf("Failed to initialize consul client: %s", err)
	}

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
		node.Attributes["consul.name"],
		node.Attributes["consul.datacenter"])

	return true, nil
}
