package openapi

import (
	"context"
	"net/http"
	"testing"

	v1 "github.com/hashicorp/nomad/testutil/openapi/v1/testclient"
)

// OpenAPIV1 exports helper methods for the V1 OpenAPI
var OpenAPIV1 = openAPIV1{}

type openAPIV1 struct{}

func (v openAPIV1) NewClientAndContext(address, port string) (*v1.APIClient, context.Context) {
	client := v1.NewAPIClient(v1.NewConfiguration())
	ctx := context.WithValue(context.Background(), v1.ContextServerVariables, map[string]string{
		"scheme":  "http",
		"address": address,
		"port":    port,
	})

	return client, ctx
}

func (v openAPIV1) ValidateMetaHeaders(r *http.Response, t *testing.T) {
	// Check for the index
	if r.Header.Get("X-Nomad-Index") == "" {
		t.Fatalf("OpenAPI response missing index")
	}
	if r.Header.Get("X-Nomad-KnownLeader") != "true" {
		t.Fatalf("OpenAPI response missing known leader")
	}
	if r.Header.Get("X-Nomad-LastContact") == "" {
		t.Fatalf("OpenAPI response missing last contact")
	}
}

func (v openAPIV1) Fatalf(funcName string, err error, t *testing.T) {
	t.Fatalf("OpenAPI client error %s: %s", funcName, err)
}
