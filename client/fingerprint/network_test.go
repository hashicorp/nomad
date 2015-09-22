package fingerprint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestNetworkFingerprint_basic(t *testing.T) {
	f := NewUnixNetworkFingerprinter(testLogger())
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

	// Darwin uses en0 for the default device, and does not have a standard
	// location for the linkspeed file, so we skip these
	if "darwin" != runtime.GOOS {
		assertNodeAttributeContains(t, node, "network.throughput")
		assertNodeAttributeContains(t, node, "network.ip-address")
	}
}

func TestNetworkFingerprint_AWS(t *testing.T) {
	// configure mock server with fixture routes, data
	// TODO: Refator with the AWS ENV test
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

	f := NewAWSNetworkFingerprinter(testLogger())
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

	assertNodeAttributeContains(t, node, "network.throughput")
	assertNodeAttributeContains(t, node, "network.ip-address")
	assertNodeAttributeContains(t, node, "network.internal-ip")
}

func TestNetworkFingerprint_notAWS(t *testing.T) {
	f := NewAWSNetworkFingerprinter(testLogger())
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
