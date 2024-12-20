// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	smithyHttp "github.com/aws/smithy-go/transport/http"

	"github.com/hashicorp/go-cleanhttp"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// AwsMetadataTimeout is the timeout used when contacting the AWS metadata
	// services.
	AwsMetadataTimeout = 2 * time.Second
)

// map of instance type to approximate speed, in Mbits/s
// Estimates from http://stackoverflow.com/a/35806587
// This data is meant for a loose approximation
var ec2NetSpeedTable = map[*regexp.Regexp]int{
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

	// used to override IMDS endpoint for testing
	endpoint string

	logger log.Logger
}

// NewEnvAWSFingerprint is used to create a fingerprint from AWS metadata
func NewEnvAWSFingerprint(logger log.Logger) Fingerprint {
	f := &EnvAWSFingerprint{
		logger: logger.Named("env_aws"),
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

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	imdsClient, err := f.imdsClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup IMDS client: %v", err)
	}

	if !isAWS(ctx, imdsClient) {
		f.logger.Debug("error querying AWS IDMS URL, skipping")
		return nil
	}

	// Keys and whether they should be namespaced as unique. Any key whose value
	// uniquely identifies a node, such as ip, should be marked as unique. When
	// marked as unique, the key isn't included in the computed node class.
	keys := map[string]bool{
		"ami-id":                      false,
		"hostname":                    true,
		"instance-id":                 true,
		"instance-life-cycle":         false,
		"instance-type":               false,
		"local-hostname":              true,
		"local-ipv4":                  true,
		"public-hostname":             true,
		"public-ipv4":                 true,
		"mac":                         true,
		"placement/availability-zone": false,
	}

	for k, unique := range keys {
		resp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: k,
		})
		if err := f.handleImdsError(err, k); err != nil {
			return err
		}
		if resp == nil {
			continue
		}

		v, err := readMetadataResponse(resp)
		if err != nil {
			return err
		}

		if v == "" {
			f.logger.Debug("read an empty value", "attribute", k)
			continue
		}

		// assume we want blank entries
		key := "platform.aws." + strings.ReplaceAll(k, "/", ".")
		if unique {
			key = structs.UniqueNamespace(key)
		}

		response.AddAttribute(key, v)
	}

	// accumulate resource information, then assign to response
	nodeResources := new(structs.NodeResources)

	// copy over network specific information
	if val, ok := response.Attributes["unique.platform.aws.local-ipv4"]; ok && val != "" {
		response.AddAttribute("unique.network.ip-address", val)
		nodeResources.Networks = []*structs.NetworkResource{
			{
				Mode:   "host",
				Device: "eth0",
				IP:     val,
				CIDR:   val + "/32",
				MBits:  f.throughput(request, imdsClient, val),
			},
		}
	}

	// copy over IPv6 network specific information
	if val, ok := response.Attributes["unique.platform.aws.mac"]; ok && val != "" {
		k := "network/interfaces/macs/" + val + "/ipv6s"
		resp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: k,
		})
		if err := f.handleImdsError(err, k); err != nil {
			return err
		}
		if resp != nil {
			addrsStr, err := readMetadataResponse(resp)
			if err != nil {
				return err
			}

			if addrsStr == "" {
				f.logger.Debug("read an empty value", "attribute", k)
			} else {
				addrs := strings.SplitN(addrsStr, "\n", 2)
				response.AddAttribute("unique.platform.aws.public-ipv6", addrs[0])
			}
		}
	}

	response.NodeResources = nodeResources

	// populate Links
	response.AddLink("aws.ec2", fmt.Sprintf("%s.%s",
		response.Attributes["platform.aws.placement.availability-zone"],
		response.Attributes["unique.platform.aws.instance-id"]))
	response.Detected = true

	return nil
}

// See https://aws.github.io/aws-sdk-go-v2/docs/handling-errors for
// recommended error handling with aws-sdk-go-v2.
// See also: https://github.com/aws/aws-sdk-go-v2/issues/1306
func (f *EnvAWSFingerprint) handleImdsError(err error, attr string) error {
	var apiErr *smithyHttp.ResponseError
	if errors.As(err, &apiErr) {
		// In the event of a request error while fetching attributes, just log and return nil.
		// This will happen if attributes do not exist for this instance (ex. ipv6, public-ipv4s).
		f.logger.Debug("could not read attribute value", "attribute", attr, "error", err)
		return nil
	}
	return err
}

func (f *EnvAWSFingerprint) instanceType(client *imds.Client) (string, error) {
	output, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{
		Path: "instance-type",
	})
	if err != nil {
		return "", err
	}
	content, err := io.ReadAll(output.Content)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func (f *EnvAWSFingerprint) throughput(request *FingerprintRequest, client *imds.Client, ip string) int {
	throughput := request.Config.NetworkSpeed
	if throughput != 0 {
		return throughput
	}

	throughput = f.linkSpeed(client)
	if throughput != 0 {
		return throughput
	}

	if request.Node.Resources != nil && len(request.Node.Resources.Networks) > 0 {
		for _, n := range request.Node.Resources.Networks {
			if n.IP == ip {
				return n.MBits
			}
		}
	}

	return defaultNetworkSpeed
}

// EnvAWSFingerprint uses lookup table to approximate network speeds
func (f *EnvAWSFingerprint) linkSpeed(client *imds.Client) int {
	instanceType, err := f.instanceType(client)
	if err != nil {
		f.logger.Error("error reading instance-type", "error", err)
		return 0
	}

	netSpeed := 0
	for reg, speed := range ec2NetSpeedTable {
		if reg.MatchString(instanceType) {
			netSpeed = speed
			break
		}
	}

	return netSpeed
}

func (f *EnvAWSFingerprint) imdsClient(ctx context.Context) (*imds.Client, error) {
	client := &http.Client{
		Transport: cleanhttp.DefaultTransport(),
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithHTTPClient(client),
		config.WithRetryMaxAttempts(0),
	)
	if err != nil {
		return nil, err
	}

	imdsClient := imds.NewFromConfig(cfg, func(o *imds.Options) {
		// endpoint should only be overridden for testing
		if f.endpoint != "" {
			o.Endpoint = f.endpoint
		}
	})
	return imdsClient, nil
}

func isAWS(ctx context.Context, client *imds.Client) bool {
	resp, err := client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "ami-id",
	})
	if err != nil {
		return false
	}

	s, err := readMetadataResponse(resp)
	if err != nil {
		return false
	}

	return s != ""
}

// readImdsResponse reads and formats the IMDS response
// and most importantly, closes the io.ReadCloser
func readMetadataResponse(resp *imds.GetMetadataOutput) (string, error) {
	defer resp.Content.Close()

	b, err := io.ReadAll(resp.Content)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
