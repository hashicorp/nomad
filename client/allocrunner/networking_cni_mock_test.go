// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"
	"net"

	"github.com/containerd/go-cni"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/hashicorp/nomad/testutil"
)

var _ cni.CNI = &mockCNIPlugin{}

type mockCNIPlugin struct {
	counter     *testutil.CallCounter
	setupErrors []string
	checkErrors []error
}

func newMockCNIPlugin() *mockCNIPlugin {

	callCounts := testutil.NewCallCounter()
	callCounts.Reset()
	return &mockCNIPlugin{
		counter:     callCounts,
		setupErrors: []string{},
		checkErrors: []error{},
	}
}

func (f *mockCNIPlugin) Setup(ctx context.Context, id, path string, opts ...cni.NamespaceOpts) (*cni.Result, error) {
	if f.counter != nil {
		f.counter.Inc("Setup")
	}
	numOfCalls := f.counter.Get()["Setup"]

	// tickle caller retries: return an error for this call number
	if numOfCalls <= len(f.setupErrors) {
		return nil, fmt.Errorf("uh oh (%d): %s", numOfCalls, f.setupErrors[numOfCalls-1])
	}

	cniResult := &cni.Result{
		Interfaces: map[string]*cni.Config{
			"eth0": {
				IPConfigs: []*cni.IPConfig{
					{
						IP: net.ParseIP("99.99.99.99"),
					},
				},
			},
		},
		// cni.Result will return a single empty struct, not an empty slice
		DNS: []types.DNS{{}},
	}
	return cniResult, nil
}

func (f *mockCNIPlugin) SetupSerially(ctx context.Context, id, path string, opts ...cni.NamespaceOpts) (*cni.Result, error) {
	return nil, nil
}

func (f *mockCNIPlugin) Remove(ctx context.Context, id, path string, opts ...cni.NamespaceOpts) error {
	return nil
}

func (f *mockCNIPlugin) Check(ctx context.Context, id, path string, opts ...cni.NamespaceOpts) error {
	if f.counter != nil {
		f.counter.Inc("Check")
	}
	numOfCalls := f.counter.Get()["Check"]
	if numOfCalls <= len(f.checkErrors) {
		return f.checkErrors[numOfCalls-1]
	}
	return nil
}

func (f *mockCNIPlugin) Status() error {
	return nil
}

func (f *mockCNIPlugin) Load(opts ...cni.Opt) error {
	return nil
}

func (f *mockCNIPlugin) GetConfig() *cni.ConfigResult {
	return nil
}
