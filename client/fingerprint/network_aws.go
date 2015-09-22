// +build linux,darwin
package fingerprint

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// AWSNetworkFingerprint is used to fingerprint the Network capabilities of a node
type AWSNetworkFingerprint struct {
	logger *log.Logger
}

// AWSNetworkFingerprint is used to create a new AWS Network Fingerprinter
func NewAWSNetworkFingerprinter(logger *log.Logger) Fingerprint {
	f := &AWSNetworkFingerprint{logger: logger}
	return f
}

func (f *AWSNetworkFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	metadataURL := os.Getenv("AWS_ENV_URL")
	if metadataURL == "" {
		metadataURL = "http://169.254.169.254/latest/meta-data/"
	}

	// assume 2 seconds is enough time for inside AWS network
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	keys := make(map[string]string)
	keys["ip-address"] = "public-hostname"
	keys["internal-ip"] = "local-ipv4"

	for name, key := range keys {
		res, err := client.Get(metadataURL + key)
		if err != nil {
			// if it's a URL error, assume we're not in an AWS environment
			if _, ok := err.(*url.Error); ok {
				return false, nil
			}
			// not sure what other errors it would return
			return false, err
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
			return false, err
		}

		// assume we want blank entries
		node.Attributes["network."+name] = strings.Trim(string(body), "\n")
	}

	if throughput := f.linkSpeed(); throughput != "" {
		node.Attributes["network.throughput"] = throughput
	}

	return true, nil
}

func (f *AWSNetworkFingerprint) linkSpeed() string {
	// This table is an approximation of network speeds based on
	// http://serverfault.com/questions/324883/aws-bandwidth-and-content-delivery/326797#326797
	// which itself cites these sources:
	// - http://blog.rightscale.com/2007/10/28/network-performance-within-amazon-ec2-and-to-amazon-s3/
	// - http://www.soc.napier.ac.uk/~bill/chris_p.pdf
	//
	// This data is meant for a loose approximation
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
