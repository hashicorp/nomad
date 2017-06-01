package fingerprint

import (
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"log"
	"net/http"
	"os"
	"time"
)

const DEFAULT_AZURE_URL = ""

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

func (f *EnvAzureFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	return false, nil
}

func (f *EnvAzureFingerprint) isAzure() bool {
	return false
}
