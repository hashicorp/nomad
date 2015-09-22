package fingerprint

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
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

	// copy over network specific items
	networkKeys := make(map[string]string)
	networkKeys["public-hostname"] = "ip-address"
	networkKeys["local-ipv4"] = "internal-ip"
	for key, name := range networkKeys {
		if node.Attributes["platform.aws."+key] != "" {
			node.Attributes["network."+name] = node.Attributes["platform.aws."+key]
		}
	}

	// find LinkSpeed from lookup
	if throughput := f.linkSpeed(); throughput != "" {
		node.Attributes["network.throughput"] = throughput
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

// EnvAWSFingerprint uses lookup table to approximate network speeds based on
// http://serverfault.com/questions/324883/aws-bandwidth-and-content-delivery/326797#326797
// which itself cites these sources:
// - http://blog.rightscale.com/2007/10/28/network-performance-within-amazon-ec2-and-to-amazon-s3/
// - http://www.soc.napier.ac.uk/~bill/chris_p.pdf
//
// This data is meant for a loose approximation
func (f *EnvAWSFingerprint) linkSpeed() string {
	net := make(map[string]string)
	net["m4.large"] = "10MB/s"
	net["m3.medium"] = "10MB/s"
	net["m3.large"] = "10MB/s"
	net["c4.large"] = "10MB/s"
	net["c3.large"] = "10MB/s"
	net["c3.xlarge"] = "10MB/s"
	net["r3.large"] = "10MB/s"
	net["r3.xlarge"] = "10MB/s"
	net["i2.xlarge"] = "10MB/s"
	net["d2.xlarge"] = "10MB/s"
	net["t2.micro"] = "2MB/s"
	net["t2.small"] = "2MB/s"
	net["t2.medium"] = "2MB/s"
	net["t2.large"] = "2MB/s"
	net["m4.xlarge"] = "95MB/s"
	net["m4.2xlarge"] = "95MB/s"
	net["m4.4xlarge"] = "95MB/s"
	net["m3.xlarge"] = "95MB/s"
	net["m3.2xlarge"] = "95MB/s"
	net["c4.xlarge"] = "95MB/s"
	net["c4.2xlarge"] = "95MB/s"
	net["c4.4xlarge"] = "95MB/s"
	net["c3.2xlarge"] = "95MB/s"
	net["c3.4xlarge"] = "95MB/s"
	net["g2.2xlarge"] = "95MB/s"
	net["r3.2xlarge"] = "95MB/s"
	net["r3.4xlarge"] = "95MB/s"
	net["i2.2xlarge"] = "95MB/s"
	net["i2.4xlarge"] = "95MB/s"
	net["d2.2xlarge"] = "95MB/s"
	net["d2.4xlarge"] = "95MB/s"
	net["m4.10xlarge"] = "10Gbp/s"
	net["c4.8xlarge"] = "10Gbp/s"
	net["c3.8xlarge"] = "10Gbp/s"
	net["g2.8xlarge"] = "10Gbp/s"
	net["r3.8xlarge"] = "10Gbp/s"
	net["i2.8xlarge"] = "10Gbp/s"
	net["d2.8xlarge"] = "10Gbp/s"

	// Query the API for the instance type, and use the table above to approximate
	// the network speed
	metadataURL := os.Getenv("AWS_ENV_URL")
	if metadataURL == "" {
		metadataURL = "http://169.254.169.254/latest/meta-data/"
	}

	// assume 2 seconds is enough time for inside AWS network
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	res, err := client.Get(metadataURL + "instance-type")
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
		return ""
	}

	key := strings.Trim(string(body), "\n")
	v, ok := net[key]
	if !ok {
		return ""
	}

	// convert to Mbps
	if strings.Contains(v, "Gbp/s") {
		i, err := strconv.Atoi(strings.TrimSuffix(v, "Gbp/s"))
		if err != nil {
			f.logger.Printf("[Err] Error converting lookup value")
			return ""
		}
		v = fmt.Sprintf("%dMB/s", i*125)
	}

	return v
}
