package csi

import (
	"context"
	"fmt"
	"testing"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/nomad/plugins/csi/fake"
	"github.com/stretchr/testify/require"
)

func newTestClient() (*fake.IdentityClient, CSIPlugin) {
	ic := &fake.IdentityClient{}
	client := &client{
		identityClient: ic,
	}

	return ic, client
}

func TestClient_RPC_PluginProbe(t *testing.T) {
	cases := []struct {
		Name             string
		ResponseErr      error
		ProbeResponse    *csipbv1.ProbeResponse
		ExpectedResponse bool
		ExpectedErr      error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "returns false for ready when the provider returns false",
			ProbeResponse: &csipbv1.ProbeResponse{
				Ready: &wrappers.BoolValue{Value: false},
			},
			ExpectedResponse: false,
		},
		{
			Name: "returns true for ready when the provider returns true",
			ProbeResponse: &csipbv1.ProbeResponse{
				Ready: &wrappers.BoolValue{Value: true},
			},
			ExpectedResponse: true,
		},
		{
			/* When a SP does not return a ready value, a CO MAY treat this as ready.
			   We do so because example plugins rely on this behaviour. We may
				 re-evaluate this decision in the future. */
			Name: "returns true for ready when the provider returns a nil wrapper",
			ProbeResponse: &csipbv1.ProbeResponse{
				Ready: nil,
			},
			ExpectedResponse: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			ic, client := newTestClient()
			defer client.Close()

			ic.NextErr = c.ResponseErr
			ic.NextPluginProbe = c.ProbeResponse

			resp, err := client.PluginProbe(context.TODO())
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			}

			require.Equal(t, c.ExpectedResponse, resp)
		})
	}

}

func TestClient_RPC_PluginInfo(t *testing.T) {
	cases := []struct {
		Name             string
		ResponseErr      error
		InfoResponse     *csipbv1.GetPluginInfoResponse
		ExpectedResponse string
		ExpectedErr      error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "returns an error if we receive an empty `name`",
			InfoResponse: &csipbv1.GetPluginInfoResponse{
				Name: "",
			},
			ExpectedErr: fmt.Errorf("PluginGetInfo: plugin returned empty name field"),
		},
		{
			Name: "returns the name when successfully retrieved and not empty",
			InfoResponse: &csipbv1.GetPluginInfoResponse{
				Name: "com.hashicorp.storage",
			},
			ExpectedResponse: "com.hashicorp.storage",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			ic, client := newTestClient()
			defer client.Close()

			ic.NextErr = c.ResponseErr
			ic.NextPluginInfo = c.InfoResponse

			resp, err := client.PluginGetInfo(context.TODO())
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			}

			require.Equal(t, c.ExpectedResponse, resp)
		})
	}

}
