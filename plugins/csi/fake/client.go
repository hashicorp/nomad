package fake

import (
	"context"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

// IdentityClient is a CSI identity client used for testing
type IdentityClient struct {
	NextErr                error
	NextPluginInfo         *csipbv1.GetPluginInfoResponse
	NextPluginCapabilities *csipbv1.GetPluginCapabilitiesResponse
	NextPluginProbe        *csipbv1.ProbeResponse
}

// NewIdentityClient returns a new IdentityClient
func NewIdentityClient() *IdentityClient {
	return &IdentityClient{}
}

func (f *IdentityClient) Reset() {
	f.NextErr = nil
	f.NextPluginInfo = nil
	f.NextPluginCapabilities = nil
	f.NextPluginProbe = nil
}

// GetPluginInfo returns plugin info
func (f *IdentityClient) GetPluginInfo(ctx context.Context, in *csipbv1.GetPluginInfoRequest, opts ...grpc.CallOption) (*csipbv1.GetPluginInfoResponse, error) {
	return f.NextPluginInfo, f.NextErr
}

// GetPluginCapabilities implements csi method
func (f *IdentityClient) GetPluginCapabilities(ctx context.Context, in *csipbv1.GetPluginCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.GetPluginCapabilitiesResponse, error) {
	return f.NextPluginCapabilities, f.NextErr
}

// Probe implements csi method
func (f *IdentityClient) Probe(ctx context.Context, in *csipbv1.ProbeRequest, opts ...grpc.CallOption) (*csipbv1.ProbeResponse, error) {
	return f.NextPluginProbe, f.NextErr
}
