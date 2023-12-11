// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package numalib

import (
	"testing"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/shoenig/test/must"
)

// badValues are example values from sysfs on unsupported platforms, e.g.,
// containers, virtualization guests
func badValues(path string) ([]byte, error) {
	return map[string][]byte{
		nodeOnline:     []byte("invalid or corrupted node online info"),
		cpuOnline:      []byte("1,3"),
		distanceFile:   []byte("invalid or corrupted distances"),
		cpulistFile:    []byte("invalid or corrupted cpu list"),
		cpuMaxFile:     []byte("3200000"),
		cpuBaseFile:    []byte("3200000"),
		cpuSocketFile:  []byte("0"),
		cpuSiblingFile: []byte("0,2"),
	}[path], nil
}

func goodValues(path string) ([]byte, error) {
	return map[string][]byte{
		nodeOnline:     []byte("0"),
		cpuOnline:      []byte("0-3"),
		distanceFile:   []byte("10"),
		cpulistFile:    []byte("0-3"),
		cpuMaxFile:     []byte("3200000"),
		cpuBaseFile:    []byte("3200000"),
		cpuSocketFile:  []byte("0"),
		cpuSiblingFile: []byte("0,2"),
	}[path], nil
}

