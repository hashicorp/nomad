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

func TestGCEFingerprint_nonGCE(t *testing.T) {
	os.Setenv("GCE_ENV_URL", "http://127.0.0.1/computeMetadata/v1/instance/")
	f := NewEnvGCEFingerprint(testLogger())
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

func testFingerprint_GCE(t *testing.T, withExternalIp bool) {
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(GCE_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in GCE ENV test: %s", err)
	}
	networkEndpoint := &endpoint{
		Uri:         "/computeMetadata/v1/instance/network-interfaces/?recursive=true",
		ContentType: "application/json",
	}
	if withExternalIp {
		networkEndpoint.Body = `[{"accessConfigs":[{"externalIp":"104.44.55.66","type":"ONE_TO_ONE_NAT"},{"externalIp":"104.44.55.67","type":"ONE_TO_ONE_NAT"}],"forwardedIps":[],"ip":"10.240.0.5","network":"projects/555555/networks/default"}]`
	} else {
		networkEndpoint.Body = `[{"accessConfigs":[],"forwardedIps":[],"ip":"10.240.0.5","network":"projects/555555/networks/default"}]`
	}
	routes.Endpoints = append(routes.Endpoints, networkEndpoint)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, ok := r.Header["Metadata-Flavor"]
		if !ok {
			t.Fatal("Metadata-Flavor not present in HTTP request header")
		}
		if value[0] != "Google" {
			t.Fatalf("Expected Metadata-Flavor Google, saw %s", value[0])
		}

		found := false
		for _, e := range routes.Endpoints {
			if r.RequestURI == e.Uri {
				w.Header().Set("Content-Type", e.ContentType)
				fmt.Fprintln(w, e.Body)
			}
			found = true
		}

		if !found {
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()
	os.Setenv("GCE_ENV_URL", ts.URL+"/computeMetadata/v1/instance/")
	f := NewEnvGCEFingerprint(testLogger())

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !ok {
		t.Fatalf("should apply")
	}

	keys := []string{
		"unique.platform.gce.id",
		"unique.platform.gce.hostname",
		"platform.gce.zone",
		"platform.gce.machine-type",
		"platform.gce.zone",
		"platform.gce.tag.abc",
		"platform.gce.tag.def",
		"unique.platform.gce.tag.foo",
		"platform.gce.attr.ghi",
		"platform.gce.attr.jkl",
		"unique.platform.gce.attr.bar",
	}

	for _, k := range keys {
		assertNodeAttributeContains(t, node, k)
	}

	if len(node.Links) == 0 {
		t.Fatalf("Empty links for Node in GCE Fingerprint test")
	}

	// Make sure Links contains the GCE ID.
	for _, k := range []string{"gce"} {
		assertNodeLinksContains(t, node, k)
	}

	assertNodeAttributeEquals(t, node, "unique.platform.gce.id", "12345")
	assertNodeAttributeEquals(t, node, "unique.platform.gce.hostname", "instance-1.c.project.internal")
	assertNodeAttributeEquals(t, node, "platform.gce.zone", "us-central1-f")
	assertNodeAttributeEquals(t, node, "platform.gce.machine-type", "n1-standard-1")
	assertNodeAttributeEquals(t, node, "platform.gce.network.default", "true")
	assertNodeAttributeEquals(t, node, "unique.platform.gce.network.default.ip", "10.240.0.5")
	if withExternalIp {
		assertNodeAttributeEquals(t, node, "unique.platform.gce.network.default.external-ip.0", "104.44.55.66")
		assertNodeAttributeEquals(t, node, "unique.platform.gce.network.default.external-ip.1", "104.44.55.67")
	} else if _, ok := node.Attributes["unique.platform.gce.network.default.external-ip.0"]; ok {
		t.Fatal("unique.platform.gce.network.default.external-ip is set without an external IP")
	}

	assertNodeAttributeEquals(t, node, "platform.gce.scheduling.automatic-restart", "TRUE")
	assertNodeAttributeEquals(t, node, "platform.gce.scheduling.on-host-maintenance", "MIGRATE")
	assertNodeAttributeEquals(t, node, "platform.gce.cpu-platform", "Intel Ivy Bridge")
	assertNodeAttributeEquals(t, node, "platform.gce.tag.abc", "true")
	assertNodeAttributeEquals(t, node, "platform.gce.tag.def", "true")
	assertNodeAttributeEquals(t, node, "unique.platform.gce.tag.foo", "true")
	assertNodeAttributeEquals(t, node, "platform.gce.attr.ghi", "111")
	assertNodeAttributeEquals(t, node, "platform.gce.attr.jkl", "222")
	assertNodeAttributeEquals(t, node, "unique.platform.gce.attr.bar", "333")
}

const GCE_routes = `
{
  "endpoints": [
    {
      "uri": "/computeMetadata/v1/instance/id",
      "content-type": "text/plain",
      "body": "12345"
    },
    {
      "uri": "/computeMetadata/v1/instance/hostname",
      "content-type": "text/plain",
      "body": "instance-1.c.project.internal"
    },
    {
      "uri": "/computeMetadata/v1/instance/zone",
      "content-type": "text/plain",
      "body": "projects/555555/zones/us-central1-f"
    },
    {
      "uri": "/computeMetadata/v1/instance/machine-type",
      "content-type": "text/plain",
      "body": "projects/555555/machineTypes/n1-standard-1"
    },
    {
      "uri": "/computeMetadata/v1/instance/tags",
      "content-type": "application/json",
      "body": "[\"abc\", \"def\", \"unique.foo\"]"
    },
    {
      "uri": "/computeMetadata/v1/instance/attributes/?recursive=true",
      "content-type": "application/json",
      "body": "{\"ghi\":\"111\",\"jkl\":\"222\",\"unique.bar\":\"333\"}"
    },
    {
      "uri": "/computeMetadata/v1/instance/scheduling/automatic-restart",
      "content-type": "text/plain",
      "body": "TRUE"
    },
    {
      "uri": "/computeMetadata/v1/instance/scheduling/on-host-maintenance",
      "content-type": "text/plain",
      "body": "MIGRATE"
    },
    {
      "uri": "/computeMetadata/v1/instance/cpu-platform",
      "content-type": "text/plain",
      "body": "Intel Ivy Bridge"
    }
  ]
}
`

func TestFingerprint_GCEWithExternalIp(t *testing.T) {
	testFingerprint_GCE(t, true)
}

func TestFingerprint_GCEWithoutExternalIp(t *testing.T) {
	testFingerprint_GCE(t, false)
}
