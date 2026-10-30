// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"testing"

	docker "github.com/moby/moby/client"
)

// DockerIsConnected checks to see if a docker daemon is available (local or remote)
func DockerIsConnected(t *testing.T) bool {

	client, err := docker.New(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}

	// Creating a client doesn't actually connect, so make sure we do something
	// like call ClientVersion() on it.
	ver := client.ClientVersion()
	t.Logf("Successfully connected to docker daemon running version %s", ver)
	return true
}

// DockerCompatible skips tests if docker is not present
func DockerCompatible(t *testing.T) {
	if !DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
}
