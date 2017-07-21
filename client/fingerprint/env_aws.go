package fingerprint

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// This is where the AWS metadata server normally resides. We hardcode the
	// "instance" path as well since it's the only one we access here.
	DEFAULT_AWS_URL = "http://169.254.169.254/latest/meta-data/"

	// AwsMetadataTimeout is the timeout used when contacting the AWS metadata
	// service
	AwsMetadataTimeout = 2 * time.Second
)

// map of instance type to approximate speed, in Mbits/s
// Estimates from http://stackoverflow.com/a/35806587
// This data is meant for a loose approximation
var ec2InstanceSpeedMap = map[*regexp.Regexp]int{
	regexp.MustCompile("t2.nano"):      30,
	regexp.MustCompile("t2.micro"):     70,
	regexp.MustCompile("t2.small"):     125,
	regexp.MustCompile("t2.medium"):    300,
	regexp.MustCompile("m3.medium"):    400,
	regexp.MustCompile("c4.8xlarge"):   4000,
	regexp.MustCompile("x1.16xlarge"):  5000,
	regexp.MustCompile(`.*\.large`):    500,
	regexp.MustCompile(`.*\.xlarge`):   750,
	regexp.MustCompile(`.*\.2xlarge`):  1000,
	regexp.MustCompile(`.*\.4xlarge`):  2000,
	regexp.MustCompile(`.*\.8xlarge`):  10000,
	regexp.MustCompile(`.*\.10xlarge`): 10000,
	regexp.MustCompile(`.*\.16xlarge`): 10000,
	regexp.MustCompile(`.*\.32xlarge`): 10000,
}

// EnvAWSFingerprint is used to fingerprint AWS metadata
type EnvAWSFingerprint struct {
	StaticFingerprinter
	timeout time.Duration
	logger  *log.Logger
}

// NewEnvAWSFingerprint is used to create a fingerprint from AWS metadata
func NewEnvAWSFingerprint(logger *log.Logger) Fingerprint {
	f := &EnvAWSFingerprint{
		logger:  logger,
		timeout: AwsMetadataTimeout,
	}
	return f
}

func (f *EnvAWSFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Check if we should tighten the timeout
	if cfg.ReadBoolDefault(TightenNetworkTimeoutsConfig, false) {
		f.timeout = 1 * time.Millisecond
	}

	if !f.isAWS() {
		return false, nil
	}

	// newNetwork is populated and addded to the Nodes resources
	newNetwork := &structs.NetworkResource{
		Device: "eth0",
	}

	if node.Links == nil {
		node.Links = make(map[string]string)
	}
	metadataURL := os.Getenv("AWS_ENV_URL")
	if metadataURL == "" {
		metadataURL = DEFAULT_AWS_URL
	}

	client := &http.Client{
		Timeout:   f.timeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	// Keys and whether they should be namespaced as unique. Any key whose value
	// uniquely identifies a node, such as ip, should be marked as unique. When
	// marked as unique, the key isn't included in the computed node class.
	keys := map[string]bool{
		"ami-id":                      true,
		"hostname":                    true,
		"instance-id":                 true,
		"instance-type":               false,
		"local-hostname":              true,
		"local-ipv4":                  true,
		"public-hostname":             true,
		"public-ipv4":                 true,
		"placement/availability-zone": false,
	}
	for k, unique := range keys {
		res, err := client.Get(metadataURL + k)
		if res.StatusCode != http.StatusOK {
			f.logger.Printf("[WARN]: fingerprint.env_aws: Could not read value for attribute %q", k)
			continue
		}
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
			f.logger.Printf("[ERR]: fingerprint.env_aws: Error reading response body for AWS %s", k)
		}

		// assume we want blank entries
		key := "platform.aws." + strings.Replace(k, "/", ".", -1)
		if unique {
			key = structs.UniqueNamespace(key)
		}

		node.Attributes[key] = strings.Trim(string(resp), "\n")
	}

	// copy over network specific information
	if val := node.Attributes["unique.platform.aws.local-ipv4"]; val != "" {
		node.Attributes["unique.network.ip-address"] = val
		newNetwork.IP = val
		newNetwork.CIDR = newNetwork.IP + "/32"
	}

	// find LinkSpeed from lookup
	throughput := f.linkSpeed()
	if cfg.NetworkSpeed != 0 {
		throughput = cfg.NetworkSpeed
	} else if throughput == 0 {
		// Failed to determine speed. Check if the network fingerprint got it
		found := false
		if node.Resources != nil && len(node.Resources.Networks) > 0 {
			for _, n := range node.Resources.Networks {
				if n.IP == newNetwork.IP {
					throughput = n.MBits
					found = true
					break
				}
			}
		}

		// Nothing detected so default
		if !found {
			throughput = defaultNetworkSpeed
		}
	}

	// populate Node Network Resources
	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}
	newNetwork.MBits = throughput
	node.Resources.Networks = []*structs.NetworkResource{newNetwork}

	// populate Links
	node.Links["aws.ec2"] = fmt.Sprintf("%s.%s",
		node.Attributes["platform.aws.placement.availability-zone"],
		node.Attributes["unique.platform.aws.instance-id"])

	return true, nil
}

func (f *EnvAWSFingerprint) isAWS() bool {
	// Read the internal metadata URL from the environment, allowing test files to
	// provide their own
	metadataURL := os.Getenv("AWS_ENV_URL")
	if metadataURL == "" {
		metadataURL = DEFAULT_AWS_URL
	}

	client := &http.Client{
		Timeout:   f.timeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	// Query the metadata url for the ami-id, to veryify we're on AWS
	resp, err := client.Get(metadataURL + "ami-id")
	if err != nil {
		f.logger.Printf("[DEBUG] fingerprint.env_aws: Error querying AWS Metadata URL, skipping")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// URL not found, which indicates that this isn't AWS
		return false
	}

	instanceID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		f.logger.Printf("[DEBUG] fingerprint.env_aws: Error reading AWS Instance ID, skipping")
		return false
	}

	match, err := regexp.MatchString("ami-*", string(instanceID))
	if err != nil || !match {
		return false
	}

	return true
}

// EnvAWSFingerprint uses lookup table to approximate network speeds
func (f *EnvAWSFingerprint) linkSpeed() int {

	// Query the API for the instance type, and use the table above to approximate
	// the network speed
	metadataURL := os.Getenv("AWS_ENV_URL")
	if metadataURL == "" {
		metadataURL = DEFAULT_AWS_URL
	}

	client := &http.Client{
		Timeout:   f.timeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	res, err := client.Get(metadataURL + "instance-type")
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		f.logger.Printf("[ERR]: fingerprint.env_aws: Error reading response body for instance-type")
		return 0
	}

	key := strings.Trim(string(body), "\n")
	netSpeed := 0
	for reg, speed := range ec2InstanceSpeedMap {
		if reg.MatchString(key) {
			netSpeed = speed
			break
		}
	}

	return netSpeed
}
