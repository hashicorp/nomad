package fingerprint

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
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
	logger  log.Logger
}

// NewEnvAWSFingerprint is used to create a fingerprint from AWS metadata
func NewEnvAWSFingerprint(logger log.Logger) Fingerprint {
	f := &EnvAWSFingerprint{
		logger:  logger.Named("env_aws"),
		timeout: AwsMetadataTimeout,
	}
	return f
}

func (f *EnvAWSFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	cfg := request.Config

	// Check if we should tighten the timeout
	if cfg.ReadBoolDefault(TightenNetworkTimeoutsConfig, false) {
		f.timeout = 1 * time.Millisecond
	}

	if !f.isAWS() {
		return nil
	}

	// newNetwork is populated and added to the Nodes resources
	newNetwork := &structs.NetworkResource{
		Device: "eth0",
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
		"ami-id":                      false,
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
			f.logger.Debug("could not read attribute value", "attribute", k)
			continue
		}
		if err != nil {
			// if it's a URL error, assume we're not in an AWS environment
			// TODO: better way to detect AWS? Check xen virtualization?
			if _, ok := err.(*url.Error); ok {
				return nil
			}
			// not sure what other errors it would return
			return err
		}
		resp, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			f.logger.Error("error reading response body for AWS attribute", "attribute", k, "error", err)
		}

		// assume we want blank entries
		key := "platform.aws." + strings.Replace(k, "/", ".", -1)
		if unique {
			key = structs.UniqueNamespace(key)
		}

		response.AddAttribute(key, strings.Trim(string(resp), "\n"))
	}

	// copy over network specific information
	if val, ok := response.Attributes["unique.platform.aws.local-ipv4"]; ok && val != "" {
		response.AddAttribute("unique.network.ip-address", val)
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
		if request.Node.Resources != nil && len(request.Node.Resources.Networks) > 0 {
			for _, n := range request.Node.Resources.Networks {
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

	newNetwork.MBits = throughput
	response.NodeResources = &structs.NodeResources{
		Networks: []*structs.NetworkResource{newNetwork},
	}

	// populate Links
	response.AddLink("aws.ec2", fmt.Sprintf("%s.%s",
		response.Attributes["platform.aws.placement.availability-zone"],
		response.Attributes["unique.platform.aws.instance-id"]))
	response.Detected = true

	return nil
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

	// Query the metadata url for the ami-id, to verify we're on AWS
	resp, err := client.Get(metadataURL + "ami-id")
	if err != nil {
		f.logger.Debug("error querying AWS Metadata URL, skipping")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// URL not found, which indicates that this isn't AWS
		return false
	}

	instanceID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		f.logger.Debug("error reading AWS Instance ID, skipping")
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
	if err != nil {
		f.logger.Error("error reading instance-type", "error", err)
		return 0
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		f.logger.Error("error reading response body for instance-type", "error", err)
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
