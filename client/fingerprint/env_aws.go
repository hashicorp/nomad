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

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

var ec2InstanceSpeedMap = map[string]int{
	"m4.large":    10,
	"m3.medium":   10,
	"m3.large":    10,
	"c4.large":    10,
	"c3.large":    10,
	"c3.xlarge":   10,
	"r3.large":    10,
	"r3.xlarge":   10,
	"i2.xlarge":   10,
	"d2.xlarge":   10,
	"t2.micro":    2,
	"t2.small":    2,
	"t2.medium":   2,
	"t2.large":    2,
	"m4.xlarge":   95,
	"m4.2xlarge":  95,
	"m4.4xlarge":  95,
	"m3.xlarge":   95,
	"m3.2xlarge":  95,
	"c4.xlarge":   95,
	"c4.2xlarge":  95,
	"c4.4xlarge":  95,
	"c3.2xlarge":  95,
	"c3.4xlarge":  95,
	"g2.2xlarge":  95,
	"r3.2xlarge":  95,
	"r3.4xlarge":  95,
	"i2.2xlarge":  95,
	"i2.4xlarge":  95,
	"d2.2xlarge":  95,
	"d2.4xlarge":  95,
	"m4.10xlarge": 1250,
	"c4.8xlarge":  1250,
	"c3.8xlarge":  1250,
	"g2.8xlarge":  1250,
	"r3.8xlarge":  1250,
	"i2.8xlarge":  1250,
	"d2.8xlarge":  1250,
}

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

	// newNetwork is populated and addded to the Nodes resources
	newNetwork := &structs.NetworkResource{}

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

	// copy over network specific information
	if node.Attributes["platform.aws.local-ipv4"] != "" {
		node.Attributes["network.ip-address"] = node.Attributes["platform.aws.local-ipv4"]
		newNetwork.IP = node.Attributes["platform.aws.local-ipv4"]
	}

	// find LinkSpeed from lookup
	if throughput := f.linkSpeed(); throughput > 0 {
		node.Attributes["network.throughput"] = fmt.Sprintf("%dMB/s", throughput)
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
func (f *EnvAWSFingerprint) linkSpeed() int {

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
		return 0
	}

	key := strings.Trim(string(body), "\n")
	v, ok := ec2InstanceSpeedMap[key]
	if !ok {
		return 0
	}

	return v
}
