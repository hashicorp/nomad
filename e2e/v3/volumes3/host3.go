// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package volumes3

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/v3/util3"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// VolumeSubmission holds state around creating and cleaning up a dynamic host
// volume.
type VolumeSubmission struct {
	t *testing.T

	nomadClient *nomadapi.Client

	// inputs
	namespace string
	filename  string
	waitState nomadapi.HostVolumeState

	// behaviors
	noCleanup bool
	timeout   time.Duration
	verbose   bool

	// outputs
	volID  string
	nodeID string
}

type Option func(*VolumeSubmission)

type Cleanup func()

func Create(t *testing.T, filename string, opts ...Option) (*VolumeSubmission, Cleanup) {
	t.Helper()

	sub := &VolumeSubmission{
		t:         t,
		namespace: api.DefaultNamespace,
		filename:  filename,
		waitState: nomadapi.HostVolumeStateReady,
		timeout:   10 * time.Second,
	}

	for _, opt := range opts {
		opt(sub)
	}

	start := time.Now()
	sub.setClient()  // setup API client if not configured by option
	sub.run(start)   // create the volume via API
	sub.waits(start) // wait on node fingerprint

	return sub, sub.cleanup
}

// VolumeID returns the volume ID set by the server
func (sub *VolumeSubmission) VolumeID() string {
	return sub.volID
}

// NodeID returns the node ID, which may have been set by the server
func (sub *VolumeSubmission) NodeID() string {
	return sub.nodeID
}

// Get fetches the api.HostVolume from the server for further examination
func (sub *VolumeSubmission) Get() *nomadapi.HostVolume {
	vol, _, err := sub.nomadClient.HostVolumes().Get(sub.volID,
		&api.QueryOptions{Namespace: sub.namespace})
	must.NoError(sub.t, err)
	return vol
}

func (sub *VolumeSubmission) setClient() {
	if sub.nomadClient != nil {
		return
	}
	nomadClient, err := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(sub.t, err, must.Sprint("failed to create nomad API client"))
	sub.nomadClient = nomadClient
}

func (sub *VolumeSubmission) run(start time.Time) {
	sub.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), sub.timeout)
	defer cancel()

	bytes, err := exec.CommandContext(ctx,
		"nomad", "volume", "create",
		"-namespace", sub.namespace,
		"-detach", sub.filename).CombinedOutput()
	must.NoError(sub.t, err, must.Sprint("error creating volume"))
	out := string(bytes)
	split := strings.Split(out, " ")
	sub.volID = strings.TrimSpace(split[len(split)-1])

	sub.logf("[%v] volume %q created", time.Since(start), sub.VolumeID())
}

func (sub *VolumeSubmission) waits(start time.Time) {
	sub.t.Helper()
	must.Wait(sub.t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			vol, _, err := sub.nomadClient.HostVolumes().Get(sub.volID,
				&api.QueryOptions{Namespace: sub.namespace})
			if err != nil {
				return err
			}
			sub.nodeID = vol.NodeID

			if vol.State != sub.waitState {
				return fmt.Errorf("volume is not yet in %q state: %q", sub.waitState, vol.State)
			}

			// if we're waiting for the volume to be ready, let's also verify
			// that it's correctly fingerprinted on the node
			switch sub.waitState {
			case nomadapi.HostVolumeStateReady:
				node, _, err := sub.nomadClient.Nodes().Info(sub.nodeID, nil)
				if err != nil {
					return err
				}
				_, ok := node.HostVolumes[vol.Name]
				if !ok {
					return fmt.Errorf("node %q did not fingerprint volume %q", sub.nodeID, sub.volID)
				}
			}

			return nil
		}),
		wait.Timeout(sub.timeout),
		wait.Gap(50*time.Millisecond),
	))

	sub.logf("[%v] volume %q is %q on node %q",
		time.Since(start), sub.volID, sub.waitState, sub.nodeID)
}

func (sub *VolumeSubmission) cleanup() {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}
	if sub.noCleanup {
		return
	}
	if sub.volID == "" {
		return
	}

	sub.noCleanup = true // so this isn't attempted more than once
	ctx, cancel := context.WithTimeout(context.Background(), sub.timeout)
	defer cancel()

	sub.logf("deleting volume %q", sub.volID)
	err := exec.CommandContext(ctx,
		"nomad", "volume", "delete",
		"-type", "host", "-namespace", sub.namespace, sub.volID).Run()
	must.NoError(sub.t, err)
}

func (sub *VolumeSubmission) logf(msg string, args ...any) {
	sub.t.Helper()
	util3.Log3(sub.t, sub.verbose, msg, args...)
}

// WithClient forces the submission to use the Nomad API client passed from the
// calling test
func WithClient(client *nomadapi.Client) Option {
	return func(sub *VolumeSubmission) {
		sub.nomadClient = client
	}
}

// WithNamespace sets a specific namespace for the volume and the wait
// query. The namespace should not be set in the spec if you're using this
// option.
func WithNamespace(ns string) Option {
	return func(sub *VolumeSubmission) {
		sub.namespace = ns
	}
}

// WithTimeout changes the default timeout from 10s
func WithTimeout(timeout time.Duration) Option {
	return func(sub *VolumeSubmission) {
		sub.timeout = timeout
	}
}

// WithWaitState changes the default state we wait for after creating the volume
// from the default of "ready"
func WithWaitState(state api.HostVolumeState) Option {
	return func(sub *VolumeSubmission) {
		sub.waitState = state
	}
}

// WithNoCleanup is used for test debugging to skip tearing down the volume
func WithNoCleanup() Option {
	return func(sub *VolumeSubmission) {
		sub.noCleanup = true
	}
}

// WithVerbose is used for test debugging to write more logs
func WithVerbose() Option {
	return func(sub *VolumeSubmission) {
		sub.verbose = true
	}
}
