package fingerprint

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

type NetworkFingerPrinter interface {
	// Fingerprint collects information about the nodes network configuration
	Fingerprint(cfg *config.Config, node *structs.Node) (bool, error)

	// Interfaces returns a slice of connected interface devices for the node
	Interfaces() []string

	// LinkSpeed queries a given interface device and returns speed information,
	// in MB/s
	LinkSpeed(device string) string
}

func NetworkDefault(logger *log.Logger) NetworkFingerPrinter {
	if isAWS() {
		return NewAWSNetworkFingerprinter(logger)
	}
	return NewNetworkFingerprinter(logger)
}

// isAWS queries the internal AWS Instance Metadata url, and determines if the
// node is running on AWS or not.
// TODO: Generalize this and use in other AWS related Fingerprinters
func isAWS() bool {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("AWS_ENV_URL")
	if metadataURL == "" {
		metadataURL = "http://169.254.169.254/latest/meta-data/"
	}

	// assume 2 seconds is enough time for inside AWS network
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Query the metadata url for the ami-id, to veryify we're on AWS
	resp, err := client.Get(metadataURL + "ami-id")

	if err != nil {
		log.Printf("[Err] Error querying AWS Metadata URL, skipping")
		return false
	}
	defer resp.Body.Close()

	instanceID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Err] Error reading AWS Instance ID, skipping")
		return false
	}

	match, err := regexp.MatchString("ami-*", string(instanceID))
	if !match {
		return false
	}

	return true
}
