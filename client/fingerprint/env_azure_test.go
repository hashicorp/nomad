package fingerprint

import "testing"

func TestEnvAzureFingerprint_nonAzure(t *testing.T) {

	t.Fatal("Die!")
}

func TestEnvAzureFingerprint_azure(t *testing.T) {

	t.Fatal("Die!")
}

type AzureRM_routes struct {
	Endpoints []*endpoint `json:"endpoints"`
}
type Azure_endpoint struct {
	Uri         string `json:"uri"`
	ContentType string `json:"content-type"`
	Body        string `json:"body"`
}

const Azure_routes = `
{
  "endpoints": [
    {
      "uri": "/computeMetadata/v1/instance/id",
      "content-type": "text/plain",
      "body": "12345"
    },
`

func TestNetworkFingerprint_Azure(t *testing.T) {
	t.Fatal("Die!")
}

func TestNetworkFingerprint_Azure_network(t *testing.T) {
	t.Fatal("Die!")
}

func TestNetworkFingerprint_notAzure(t *testing.T) {
	t.Fatal("Die!")
}
