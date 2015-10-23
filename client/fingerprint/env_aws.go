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

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// map of instance type to approximate speed, in Mbits/s
// http://serverfault.com/questions/324883/aws-bandwidth-and-content-delivery/326797#326797
// which itself cites these sources:
// - http://blog.rightscale.com/2007/10/28/network-performance-within-amazon-ec2-and-to-amazon-s3/
// - http://www.soc.napier.ac.uk/~bill/chris_p.pdf
//
// This data is meant for a loose approximation
var ec2InstanceSpeedMap = map[string]int{
	"m4.large":    80,
	"m3.medium":   80,
	"m3.large":    80,
	"c4.large":    80,
	"c3.large":    80,
	"c3.xlarge":   80,
	"r3.large":    80,
	"r3.xlarge":   80,
	"i2.xlarge":   80,
	"d2.xlarge":   80,
	"t2.micro":    16,
	"t2.small":    16,
	"t2.medium":   16,
	"t2.large":    16,
	"m4.xlarge":   760,
	"m4.2xlarge":  760,
	"m4.4xlarge":  760,
	"m3.xlarge":   760,
	"m3.2xlarge":  760,
	"c4.xlarge":   760,
	"c4.2xlarge":  760,
	"c4.4xlarge":  760,
	"c3.2xlarge":  760,
	"c3.4xlarge":  760,
	"g2.2xlarge":  760,
	"r3.2xlarge":  760,
	"r3.4xlarge":  760,
	"i2.2xlarge":  760,
	"i2.4xlarge":  760,
	"d2.2xlarge":  760,
	"d2.4xlarge":  760,
	"m4.10xlarge": 10000,
	"c4.8xlarge":  10000,
	"c3.8xlarge":  10000,
	"g2.8xlarge":  10000,
	"r3.8xlarge":  10000,
	"i2.8xlarge":  10000,
	"d2.8xlarge":  10000,
}

// EnvAWSFingerprint is used to fingerprint AWS metadata
type EnvAWSFingerprint struct {
	logger *log.Logger
}

// NewEnvAWSFingerprint is used to create a fingerprint from AWS metadata
func NewEnvAWSFingerprint(logger *log.Logger) Fingerprint {
	f := &EnvAWSFingerprint{logger: logger}
	return f
}

func (f *EnvAWSFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	if !isAWS() {
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
		metadataURL = "http://169.254.169.254/latest/meta-data/"
	}

	// assume 2 seconds is enough time for inside AWS network
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: cleanhttp.DefaultTransport(),
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
			f.logger.Printf("[ERR]: fingerprint.env_aws: Error reading response body for AWS %s", k)
		}

		// assume we want blank entries
		key := strings.Replace(k, "/", ".", -1)
		node.Attributes["platform.aws."+key] = strings.Trim(string(resp), "\n")
	}

	// copy over network specific information
	if node.Attributes["platform.aws.local-ipv4"] != "" {
		node.Attributes["network.ip-address"] = node.Attributes["platform.aws.local-ipv4"]
		newNetwork.IP = node.Attributes["platform.aws.local-ipv4"]
		newNetwork.CIDR = newNetwork.IP + "/32"
	}

	// find LinkSpeed from lookup
	if throughput := f.linkSpeed(); throughput > 0 {
		newNetwork.MBits = throughput
	}

	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}
	node.Resources.Networks = append(node.Resources.Networks, newNetwork)

	// populate Node Network Resources

	// populate Links
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
		Timeout:   2 * time.Second,
		Transport: cleanhttp.DefaultTransport(),
	}

	// Query the metadata url for the ami-id, to veryify we're on AWS
	resp, err := client.Get(metadataURL + "ami-id")

	if err != nil {
		log.Printf("[ERR] fingerprint.env_aws: Error querying AWS Metadata URL, skipping")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// URL not found, which indicates that this isn't AWS
		return false
	}

	instanceID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[ERR] fingerprint.env_aws: Error reading AWS Instance ID, skipping")
		return false
	}

	match, err := regexp.MatchString("ami-*", string(instanceID))
	if !match {
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
		metadataURL = "http://169.254.169.254/latest/meta-data/"
	}

	// assume 2 seconds is enough time for inside AWS network
	client := &http.Client{
		Timeout:   2 * time.Second,
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
	v, ok := ec2InstanceSpeedMap[key]
	if !ok {
		return 0
	}

	return v
}
