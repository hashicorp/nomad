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

// badSysData are example values from sysfs on unsupported platforms, e.g.,
// containers, virtualization guests
func badSysData(path string) ([]byte, error) {
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

func goodSysData(path string) ([]byte, error) {
	return map[string][]byte{
		nodeOnline:     []byte("0-3"),
		cpuOnline:      []byte("0-3"),
		distanceFile:   []byte("10"),
		cpulistFile:    []byte("0-3"),
		cpuMaxFile:     []byte("3200000"),
		cpuBaseFile:    []byte("3200000"),
		cpuSocketFile:  []byte("0"),
		cpuSiblingFile: []byte("0,2"),
	}[path], nil
}

func TestSysfs_discoverOnline(t *testing.T) {
	st := NewTopology(&idset.Set[hw.NodeID]{}, SLIT{}, []Core{})
	goodIDSet := idset.From[hw.NodeID]([]uint8{0, 1, 2, 3})

	tests := []struct {
		name          string
		readerFunc    pathReaderFn
		expectedIDSet *idset.Set[hw.NodeID]
	}{
		{"lxc values", badSysData, idset.Empty[hw.NodeID]()},
		{"good values", goodSysData, goodIDSet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sy := &Sysfs{}
			sy.discoverOnline(st, tt.readerFunc)
			must.Eq(t, tt.expectedIDSet, st.NodeIDs)
		})
	}
}

func TestSysfs_discoverCosts(t *testing.T) {
	st := NewTopology(idset.Empty[hw.NodeID](), SLIT{}, []Core{})
	fourNodes := idset.From[hw.NodeID]([]uint8{0, 1, 2, 3})
	twoNodes := idset.From[hw.NodeID]([]uint8{1, 3})

	tests := []struct {
		name              string
		nodeIDs           *idset.Set[hw.NodeID]
		readerFunc        pathReaderFn
		expectedDistances SLIT
	}{
		{"empty node IDs", idset.Empty[hw.NodeID](), os.ReadFile, SLIT{}},
		{"four nodes and bad sys data", fourNodes, badSysData, SLIT{
			[]Cost{0, 0, 0, 0},
			[]Cost{0, 0, 0, 0},
			[]Cost{0, 0, 0, 0},
			[]Cost{0, 0, 0, 0},
		}},
		{"two nodes and good sys data", twoNodes, goodSysData, SLIT{
			[]Cost{0, 0},
			[]Cost{0, 0},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sy := &Sysfs{}
			st.NodeIDs = tt.nodeIDs
			sy.discoverCosts(st, tt.readerFunc)
			must.Eq(t, tt.expectedDistances, st.Distances)
		})
	}
}

