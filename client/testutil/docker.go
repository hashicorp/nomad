// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"runtime"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/testutil"
)

// DockerIsConnected checks to see if a docker daemon is available (local or remote)
func DockerIsConnected(t *testing.T) bool {
	// We have docker on travis so we should try to test
	if testutil.IsTravis() {
		// Travis supports Docker on Linux only; MacOS setup does not support Docker
		return runtime.GOOS == "linux"
	}

	if testutil.IsAppVeyor() {
		return runtime.GOOS == "windows"
	}

	client, err := docker.NewClientFromEnv()
	if err != nil {
		return false
	}

	// Creating a client doesn't actually connect, so make sure we do something
	// like call Version() on it.
	env, err := client.Version()
	if err != nil {
		t.Logf("Failed to connect to docker daemon: %s", err)
		return false
	}

	t.Logf("Successfully connected to docker daemon running version %s", env.Get("Version"))
	return true
}

// DockerCompatible skips tests if docker is not present
func DockerCompatible(t *testing.T) {
	if !DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
}
