package fingerprint

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// EnvAWSFingerprint is used to fingerprint the CPU
type EnvAWSFingerprint struct {
	logger *log.Logger
}

// NewEnvAWSFingerprint is used to create a CPU fingerprint
func NewEnvAWSFingerprint(logger *log.Logger) Fingerprint {
	f := &EnvAWSFingerprint{logger: logger}
	return f
}

func (f *EnvAWSFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	if !isAWS() {
		return false, nil
	}
	if node.Links == nil {
		node.Links = make(map[string]string)
	}
	metadataURL := os.Getenv("AWS_ENV_URL")
	if metadataURL == "" {
		metadataURL = "http://169.254.169.254/latest/meta-data/"
	}

	// assume 2 seconds is enough time for inside AWS network
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	keys := []string{
		"ami-id",
		"hostname",
		"instance-id",
		"instance-type",
		"local-hostname",
		"local-ipv4",
		"public-hostname",
		"public-ipv4",
		"placement/availability-zone",
	}
	for _, k := range keys {
		res, err := client.Get(metadataURL + k)
		if err != nil {
			// if it's a URL error, assume we're not in an AWS environment
			// TODO: better way to detect AWS? Check xen virtualization?
			if _, ok := err.(*url.Error); ok {
				return false, nil
			}
			// not sure what other errors it would return
			return false, err
		}
		resp, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}

		// assume we want blank entries
		key := strings.Replace(k, "/", ".", -1)
		node.Attributes["platform.aws."+key] = strings.Trim(string(resp), "\n")
	}

	// populate links
	node.Links["aws.ec2"] = node.Attributes["platform.aws.placement.availability-zone"] + "." + node.Attributes["platform.aws.instance-id"]

	return true, nil
}

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
