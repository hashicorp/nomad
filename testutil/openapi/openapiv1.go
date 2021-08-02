package openapi

import (
	"context"
	v1 "github.com/hashicorp/nomad/testutil/openapi/v1/testclient"
	"net/http"
	"testing"
)

// OpenAPIV1 exports helper methods for the V1 OpenAPI
var OpenAPIV1 = openAPIV1{}

type openAPIV1 struct{}

func (v openAPIV1) NewClientAndContext(protocol, port string) (*v1.APIClient, context.Context) {
	client := v1.NewAPIClient(v1.NewConfiguration())
	ctx := context.WithValue(context.Background(), v1.ContextServerVariables, map[string]string{
		"protocol": protocol,
		"port":     port,
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
