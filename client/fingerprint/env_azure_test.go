package fingerprint

import (
	"testing"
	"os"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/client/config"
	"hashicorp/packer/common/json"
	"net/http/httptest"
	"net/http"
	"fmt"
	"time"
	"github.com/hashicorp/go-cleanhttp"
	"net/url"
	"io/ioutil"
)

func TestEnvAzureFingerprint_nonAzure(t *testing.T) {
	os.Setenv("AZURE_ENV_URL", "http://localhost/metadata/instance?api-version=2017-03-01")
	f := NewEnvAzureFingerprint(testLogger())

	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)

	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ok {
		t.Fatal("Should not reach here without mock server running!")
	}
}

func TestEnvAzureFingerprint_azure(t *testing.T) {
	t.Fatal("Die!")
}

type AzureRM_routes struct {
	Endpoints []*Azure_endpoint `json:"endpoints"`
}
type Azure_endpoint struct {
	Uri         string `json:"uri"`
	Header      string `json:"header"`
	ContentType string `json:"content-type"`
	HTTPStatus  string `json:http-status`
	Body        string `json:"body"`
}

// In order to investigate the scenarios; you can use the curl command as per below (NOTE: -i for header):
// curl -i  --header "Metadata:true"  "http://169.254.169.254/metadata?api-version=2017-03-01&format=text"
const Azure_routes = `
{
  "endpoints": [
    {
      "uri": "/metadata/instance/compute/location?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "SouthEastAsia"
    },
    {
      "uri": "/metadata/instance/compute/osType?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "Linux"
    },
    {
      "uri": "/metadata/instance/compute/platformUpdateDomain?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "1"
    },
    {
      "uri": "/metadata/instance/compute/platformFaultDomain?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "5"
    },
    {
      "uri": "/metadata/instance/compute/vmId?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "5c08b38e-4d57-4c23-ac45-aca61037f084"
    },
    {
      "uri": "/metadata/instance/compute/vmSize?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "Standard_DS2"
    },
    {
      "uri": "/metadata/instance/network/interface/0/ipv4/ipaddress/0/ipaddress?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "10.0.0.4"
    },
    {
      "uri": "/metadata/instance/network/interface/0/ipv4/ipaddress/0/publicip?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "54.128.200.22"
    },
    {
      "uri": "/metadata/instance/network/interface/0/ipv4/subnet/0/prefix?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "24"
    },
    {
      "uri": "/metadata/instance/network/interface/0/ipv4/subnet/0/address?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "10.0.101.0"
    },
    {
      "uri": "/metadata/instance/network/interface/0/ipv4/subnet/0/dnsservers/1/ipaddress?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "10.0.0.3"
    },
    {
      "uri": "/metadata/instance/network/interface/0/mac?api-version=2017-03-01&format=text",
      "header": "Metadata:true",
      "content-type": "text/plain",
      "http-status": "200",
      "body": "000D3A00FA89"
    },
    {
      "uri": "/metadata/scheduledevents?api-version=2017-03-01",
      "header": "Metadata:true",
      "content-type": "application/json",
      "http-status": "200",
      "body": "{\"DocumentIncarnation\":0, \"Events\":[]}"
    },
    {
      "uri": "/metadata/instance/unknown?api-version=2017-03-01",
      "header": "Metadata:true",
      "content-type": "application/json",
      "http-status": "404",
      "body": "{ \"error\": \"Not found\" }"
    }
  ]
}
`

func TestNetworkFingerprint_Azure(t *testing.T) {
	t.Fatal("Die!")
}

func TestNetworkFingerprint_Azure_network(t *testing.T) {
	t.Fatal("Die!")
}

// Correctly catches that it is not OK if it is not a valid Azure metadata endpoint
func TestNetworkFingerprint_notAzure(t *testing.T) {

	// TODO: Should do more of Network testing ..
	os.Setenv("AZURE_ENV_URL", "http://localhost/metadata/instance?api-version=2017-03-01")
	f := NewEnvAzureFingerprint(testLogger())

	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)

	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ok {
		t.Fatal("Should not reach here as it is not a valid Azure metadata endpoint!")
	}
}

func testFingerprint_Azure(t *testing.T, withExternalIp bool) {
	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(Azure_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshall JSON in Azure ENV test: %s", err)
	}

	for _, e := range routes.Endpoints {
		fmt.Printf("%v", e)
	}

}

func TestFingerprint_AzureWithExternalIp(t *testing.T) {
	testFingerprint_Azure(t, true)
}

func TestFingerprint_AzureWithoutExternalIp(t *testing.T) {
	testFingerprint_Azure(t, false)
}

func TestAzureMetadataEndpoint(t *testing.T) {

	// Setup client and legal + illegal EnvAzureFingerprint
	// assume 2 seconds is enough time for inside Azure network
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: cleanhttp.DefaultTransport(),
	}

	// isAzure without test server
	f_not_azure := &EnvAzureFingerprint{
		client:      client,
		logger:      testLogger(),
		metadataURL: "http://localhost/metadata?api-version=" + DEFAULT_AZURE_API_VERSION + "&format=text",
	}

	if f_not_azure.isAzure() != false {
		t.Fatal("WRONG!!")
	}

	// Now for tests with server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For test; only text output returned
		w.Header().Set("Content-Type", "text/plain")
		if r.Header.Get("Metadata") != "true" {
			// Needs to be set; otherwise it is Bad Request
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Bad request. Required metadata header not specified")
		} else if r.Header.Get("X-Forward-For") != "" {
			// Cannot forward via Proxy ..
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Bad Request")
		} else {
			// For isAzure test
			if r.RequestURI == "/metadata?api-version=2017-03-01&format=text" {
				fmt.Fprint(w, "instance/")
			}
		}
	}))
	defer ts.Close()

	f := &EnvAzureFingerprint{
		client:      client,
		logger:      testLogger(),
		metadataURL: ts.URL + "/metadata?api-version=" + DEFAULT_AZURE_API_VERSION + "&format=text",
	}

	parsedUrl, err := url.Parse(f.metadataURL)
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}

	missing_metadata_req := &http.Request{
		Method: "GET",
		URL:    parsedUrl,
		Header: http.Header{},
	}

	if res, err := f.client.Do(missing_metadata_req); err != nil {
		t.Fatalf("err: %v", err)
	} else {
		mybody, _ := ioutil.ReadAll(res.Body)
		f.logger.Printf("Body: %v", string(mybody))
		// If not passing the correct header for Metadata; it should be caught
		if res.StatusCode != http.StatusBadRequest {
			t.Fatal("Should catch if no metadata header being passed")
		}
	}

	illegal_forward_req := &http.Request{
		Method: "GET",
		URL:    parsedUrl,
		Header: http.Header{
			"Metadata":      []string{"true"},
			"X-Forward-For": []string{"202.188.1.3"},
		},
	}
	if res, err := f.client.Do(illegal_forward_req); err != nil {
		t.Fatalf("err: %v", err)
	} else {
		mybody, _ := ioutil.ReadAll(res.Body)
		f.logger.Printf("Body: %v", string(mybody))
		// If not passing the correct header for Metadata; it should be caught
		if res.StatusCode != http.StatusBadRequest {
			t.Fatal("Should catch if no metadata being passed")
		}
	}

	// Final test; with real server
	if f.isAzure() != true {
		t.Fatal("WRONG!")
	}
}
