package testutil

import (
	"context"
	v1 "github.com/hashicorp/nomad/openapiv1"
)

func NewOpenAPIClientAndContext(protocol, port string) (*v1.APIClient, context.Context) {
	client := v1.NewAPIClient(v1.NewConfiguration())
	ctx := context.WithValue(context.Background(), v1.ContextServerVariables, map[string]string{
		"protocol": protocol,
		"port": port,
	})

	return client, ctx
}
