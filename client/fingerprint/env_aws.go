package fingerprint

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/hashicorp/go-hclog"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
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

	// endpoint for EC2 metadata as expected by AWS SDK
	endpoint string

	logger log.Logger
}

// NewEnvAWSFingerprint is used to create a fingerprint from AWS metadata
func NewEnvAWSFingerprint(logger log.Logger) Fingerprint {
	f := &EnvAWSFingerprint{
		logger:   logger.Named("env_aws"),
		endpoint: strings.TrimSuffix(os.Getenv("AWS_ENV_URL"), "/meta-data/"),
	}
	return f
}

func (f *EnvAWSFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	cfg := request.Config

	timeout := AwsMetadataTimeout

	// Check if we should tighten the timeout
	if cfg.ReadBoolDefault(TightenNetworkTimeoutsConfig, false) {
		timeout = 1 * time.Millisecond
	}

	ec2meta, err := ec2MetaClient(f.endpoint, timeout)
	if err != nil {
		return fmt.Errorf("failed to setup ec2Metadata client: %v", err)
	}

	if !ec2meta.Available() {
		return nil
	}

	// newNetwork is populated and added to the Nodes resources
	newNetwork := &structs.NetworkResource{
		Device: "eth0",
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
		resp, err := ec2meta.GetMetadata(k)
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			f.logger.Debug("could not read attribute value", "attribute", k, "error", awsErr)
			continue
		} else if awsErr, ok := err.(awserr.Error); ok {
			// if it's a URL error, assume we're not in an AWS environment
			// TODO: better way to detect AWS? Check xen virtualization?
			if _, ok := awsErr.OrigErr().(*url.Error); ok {
				return nil
			}

			// not sure what other errors it would return
			return err
		}

		// assume we want blank entries
		key := "platform.aws." + strings.Replace(k, "/", ".", -1)
		if unique {
			key = structs.UniqueNamespace(key)
		}

		response.AddAttribute(key, strings.Trim(resp, "\n"))
	}

	// copy over network specific information
	if val, ok := response.Attributes["unique.platform.aws.local-ipv4"]; ok && val != "" {
		response.AddAttribute("unique.network.ip-address", val)
		newNetwork.IP = val
		newNetwork.CIDR = newNetwork.IP + "/32"
	}

	// find LinkSpeed from lookup
	throughput := cfg.NetworkSpeed
	if throughput == 0 {
		throughput = f.linkSpeed(ec2meta)
	}
	if throughput == 0 {
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

// EnvAWSFingerprint uses lookup table to approximate network speeds
func (f *EnvAWSFingerprint) linkSpeed(ec2meta *ec2metadata.EC2Metadata) int {

	resp, err := ec2meta.GetMetadata("instance-type")
	if err != nil {
		f.logger.Error("error reading instance-type", "error", err)
		return 0
	}

	key := strings.Trim(resp, "\n")
	netSpeed := 0
	for reg, speed := range ec2InstanceSpeedMap {
		if reg.MatchString(key) {
			netSpeed = speed
			break
		}
	}

	return netSpeed
}

func ec2MetaClient(endpoint string, timeout time.Duration) (*ec2metadata.EC2Metadata, error) {
	client := &http.Client{
		Timeout:   timeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	c := aws.NewConfig().WithHTTPClient(client).WithMaxRetries(0)
	if endpoint != "" {
		c = c.WithEndpoint(endpoint)
	}

	session, err := session.NewSession(c)
	if err != nil {
		return nil, err
	}
	return ec2metadata.New(session, c), nil
}
