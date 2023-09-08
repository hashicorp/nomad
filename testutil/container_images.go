// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"os"
	"runtime"
)

const (
	// BusyboxImageEnvName is always used, if set, by TestBusyboxImage()
	BusyboxImageEnvName = "BUSYBOX_IMAGE"
)

func TestDockerImage(name, tag string) string {
	img := name + ":" + tag
	if IsCI() {
		// use our mirror to avoid rate-limiting in CI
		img = "docker.mirror.hashicorp.services/" + img
	} else {
		// explicitly include docker.io for podman
		img = "docker.io/" + img
	}
	return img
}

func TestBusyboxImage() string {
	// if env is set, use it verbatim
	if img, ok := os.LookupEnv(BusyboxImageEnvName); ok {
		return img
	}

	// otherwise, figure it out
	name := "busybox"
	tag := "1"
	if runtime.GOOS == "windows" {
		// this image is maintained in https://github.com/hashicorp/busybox-windows
		name = "hashicorpdev/busybox-windows"
		tag = "server2016-0.1"
	}
	return TestDockerImage(name, tag)
}
