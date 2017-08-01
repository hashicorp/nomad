package fingerprint

import (
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"log"
	"net/http"
	"os"
	"time"
	"fmt"
)

// Details: https://docs.microsoft.com/en-us/azure/virtual-machines/virtual-machines-instancemetadataservice-overview
const DEFAULT_AZURE_API_VERSION = "2017-03-01"
const DEFAULT_AZURE_URL = "http://169.254.169.254/metadata/instance?api-version=" + DEFAULT_AZURE_API_VERSION

type AzureMetadataNetworkInterface struct {
	AccessConfigs []struct {
		ExternalIp string
		Type       string
	}
	ForwardedIps []string
	Ip           string
	Network      string
}

type AzureReqError struct {
	StatusCode int
}

func (e AzureReqError) Error() string {
	return http.StatusText(e.StatusCode)
}

type EnvAzureFingerprint struct {
	StaticFingerprinter
	client      *http.Client
	logger      *log.Logger
	metadataURL string
}

func NewEnvAzureFingerprint(logger *log.Logger) Fingerprint {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("AZURE_ENV_URL")
	if metadataURL == "" {
		metadataURL = DEFAULT_AZURE_URL
	}

	// assume 2 seconds is enough time for inside Azure network
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: cleanhttp.DefaultTransport(),
	}

	return &EnvAzureFingerprint{
		client:      client,
		logger:      logger,
		metadataURL: metadataURL,
	}
}

func (f *EnvAzureFingerprint) Get(attribute string, isText bool) {
	// By default; return JSON

	// unless specifically mentioned to be Text

}
// Mapping of Azure Compute Units (ACUs); IO, Network refer to below:
// https://docs.microsoft.com/en-us/azure/virtual-machines/virtual-machines-windows-sizes
func (f *EnvAzureFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {

	if !f.isAzure() {
		return false, nil
	}

	if node.Links == nil {
		node.Links = make(map[string]string)
	}
	// Keys and whether they should be namespaced as unique. Any key whose value
	// uniquely identifies a node, such as ip, should be marked as unique. When
	// marked as unique, the key isn't included in the computed node class.
	keys := map[string]bool{
		"vm-id":                          true,
		"vm-size":                        false,
		"os-type":                        false,
		"local-ipv4":                     true,
		"public-ipv4":                    true,
		"placement/update-domain":        false,
		"placement/fault-domain":         false,
		"scheduling/automatic-restart":   false,
		"scheduling/on-host-maintenance": false,
	}

	fmt.Printf("%v", keys)

	return false, nil
}

func (f *EnvAzureFingerprint) isAzure() bool {
	// Call the instance name; if returns something; consider that it is proxy for Azure
	// any other way?

	f.client.Get()
	return false
}
