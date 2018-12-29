package fingerprint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestEnvAWSFingerprint_nonAws(t *testing.T) {
	os.Setenv("AWS_ENV_URL", "http://127.0.0.1/latest/meta-data/")
	f := NewEnvAWSFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ok {
		t.Fatalf("Should be false without test server")
	}
}

func TestEnvAWSFingerprint_aws(t *testing.T) {
	f := NewEnvAWSFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(aws_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in AWS ENV test: %s", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, e := range routes.Endpoints {
			if r.RequestURI == e.Uri {
				w.Header().Set("Content-Type", e.ContentType)
				fmt.Fprintln(w, e.Body)
			}
		}
	}))
	defer ts.Close()
	os.Setenv("AWS_ENV_URL", ts.URL+"/latest/meta-data/")

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !ok {
		t.Fatalf("Expected AWS attributes and Links")
	}

	keys := []string{
		"platform.aws.ami-id",
		"unique.platform.aws.hostname",
		"unique.platform.aws.instance-id",
		"platform.aws.instance-type",
		"unique.platform.aws.local-hostname",
		"unique.platform.aws.local-ipv4",
		"unique.platform.aws.public-hostname",
		"unique.platform.aws.public-ipv4",
		"platform.aws.placement.availability-zone",
		"unique.network.ip-address",
	}

	for _, k := range keys {
		assertNodeAttributeContains(t, node, k)
	}

	if len(node.Links) == 0 {
		t.Fatalf("Empty links for Node in AWS Fingerprint test")
	}

	// confirm we have at least instance-id and ami-id
	for _, k := range []string{"aws.ec2"} {
		assertNodeLinksContains(t, node, k)
	}
}

type routes struct {
	Endpoints []*endpoint `json:"endpoints"`
}
type endpoint struct {
	Uri         string `json:"uri"`
	ContentType string `json:"content-type"`
	Body        string `json:"body"`
}

const aws_routes = `
{
  "endpoints": [
    {
      "uri": "/latest/meta-data/ami-id",
      "content-type": "text/plain",
      "body": "ami-1234"
    },
    {
      "uri": "/latest/meta-data/hostname",
      "content-type": "text/plain",
      "body": "ip-10-0-0-207.us-west-2.compute.internal"
    },
    {
      "uri": "/latest/meta-data/placement/availability-zone",
      "content-type": "text/plain",
      "body": "us-west-2a"
    },
    {
      "uri": "/latest/meta-data/instance-id",
      "content-type": "text/plain",
      "body": "i-b3ba3875"
    },
    {
      "uri": "/latest/meta-data/instance-type",
      "content-type": "text/plain",
      "body": "m3.2xlarge"
    },
    {
      "uri": "/latest/meta-data/local-hostname",
      "content-type": "text/plain",
      "body": "ip-10-0-0-207.us-west-2.compute.internal"
    },
    {
      "uri": "/latest/meta-data/local-ipv4",
      "content-type": "text/plain",
      "body": "10.0.0.207"
    },
    {
      "uri": "/latest/meta-data/public-hostname",
      "content-type": "text/plain",
      "body": "ec2-54-191-117-175.us-west-2.compute.amazonaws.com"
    },
    {
      "uri": "/latest/meta-data/public-ipv4",
      "content-type": "text/plain",
      "body": "54.191.117.175"
    }
  ]
}
`

func TestNetworkFingerprint_AWS(t *testing.T) {
	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(aws_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in AWS ENV test: %s", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, e := range routes.Endpoints {
			if r.RequestURI == e.Uri {
				w.Header().Set("Content-Type", e.ContentType)
				fmt.Fprintln(w, e.Body)
			}
		}
	}))

	defer ts.Close()
	os.Setenv("AWS_ENV_URL", ts.URL+"/latest/meta-data/")

	f := NewEnvAWSFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}

	assertNodeAttributeContains(t, node, "unique.network.ip-address")

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := node.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to have an IP")
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}
}

func TestNetworkFingerprint_AWS_network(t *testing.T) {
	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(aws_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in AWS ENV test: %s", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, e := range routes.Endpoints {
			if r.RequestURI == e.Uri {
				w.Header().Set("Content-Type", e.ContentType)
				fmt.Fprintln(w, e.Body)
			}
		}
	}))

	defer ts.Close()
	os.Setenv("AWS_ENV_URL", ts.URL+"/latest/meta-data/")

	f := NewEnvAWSFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	cfg := &config.Config{}
	ok, err := f.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}

	assertNodeAttributeContains(t, node, "unique.network.ip-address")

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := node.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to have an IP")
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}
	if net.MBits != 1000 {
		t.Fatalf("Expected Network Resource to have speed %d; got %d", 1000, net.MBits)
	}

	// Try again this time setting a network speed in the config
	node = &structs.Node{
		Attributes: make(map[string]string),
	}

	cfg.NetworkSpeed = 10
	ok, err = f.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}

	assertNodeAttributeContains(t, node, "unique.network.ip-address")

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net = node.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to have an IP")
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}
	if net.MBits != 10 {
		t.Fatalf("Expected Network Resource to have speed %d; got %d", 10, net.MBits)
	}
}

func TestNetworkFingerprint_notAWS(t *testing.T) {
	os.Setenv("AWS_ENV_URL", "http://127.0.0.1/latest/meta-data/")
	f := NewEnvAWSFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("Should not apply")
	}
}
