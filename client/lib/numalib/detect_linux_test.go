// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package numalib

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/shoenig/test/must"
)

// badSysData are example values from sysfs on unsupported platforms, e.g.,
// containers, virtualization guests
func badSysData(path string) ([]byte, error) {
	return map[string][]byte{
		nodeOnline:     []byte("invalid data"),
		cpuOnline:      []byte("1,3"),
		distanceFile:   []byte("invalid data"),
		cpulistFile:    []byte("invalid data"),
		cpuMaxFile:     []byte("3200000"),
		cpuBaseFile:    []byte("3200000"),
		cpuSocketFile:  []byte("0"),
		cpuSiblingFile: []byte("0,2"),
	}[path], nil
}

func goodSysData(path string) ([]byte, error) {
	return map[string][]byte{
		"/sys/devices/system/node/online":                            []byte("0-1"),
		"/sys/devices/system/cpu/online":                             []byte("0-3"),
		"/sys/devices/system/node0/distance":                         []byte("10"),
		"/sys/devices/system/node0/cpulist":                          []byte("0-3"),
		"/sys/devices/system/node1/distance":                         []byte("10"),
		"/sys/devices/system/node1/cpulist":                          []byte("0-3"),
		"/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq":      []byte("3200000"),
		"/sys/devices/system/cpu/cpu0/cpufreq/base_frequency":        []byte("3200000"),
		"/sys/devices/system/cpu/cpu0/topology/physical_package_id":  []byte("0"),
		"/sys/devices/system/cpu/cpu0/topology/thread_siblings_list": []byte("0,2"),
		"/sys/devices/system/cpu/cpu1/cpufreq/cpuinfo_max_freq":      []byte("3200000"),
		"/sys/devices/system/cpu/cpu1/cpufreq/base_frequency":        []byte("3200000"),
		"/sys/devices/system/cpu/cpu1/topology/physical_package_id":  []byte("0"),
		"/sys/devices/system/cpu/cpu1/topology/thread_siblings_list": []byte("0,2"),
		"/sys/devices/system/cpu/cpu2/cpufreq/cpuinfo_max_freq":      []byte("3200000"),
		"/sys/devices/system/cpu/cpu2/cpufreq/base_frequency":        []byte("3200000"),
		"/sys/devices/system/cpu/cpu2/topology/physical_package_id":  []byte("0"),
		"/sys/devices/system/cpu/cpu2/topology/thread_siblings_list": []byte("0,2"),
		"/sys/devices/system/cpu/cpu3/cpufreq/cpuinfo_max_freq":      []byte("3200000"),
		"/sys/devices/system/cpu/cpu3/cpufreq/base_frequency":        []byte("3200000"),
		"/sys/devices/system/cpu/cpu3/topology/physical_package_id":  []byte("0"),
		"/sys/devices/system/cpu/cpu3/topology/thread_siblings_list": []byte("0,2"),
	}[path], nil
}

func TestSysfs_discoverOnline(t *testing.T) {
	st := NewTopology(&idset.Set[hw.NodeID]{}, SLIT{}, []Core{})
	goodIDSet := idset.From[hw.NodeID]([]uint8{0, 1})

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
	twoNodes := idset.From[hw.NodeID]([]uint8{1, 3})

	tests := []struct {
		name              string
		nodeIDs           *idset.Set[hw.NodeID]
		readerFunc        pathReaderFn
		expectedDistances SLIT
	}{
		{"empty node IDs", idset.Empty[hw.NodeID](), os.ReadFile, SLIT{}},
		{"two nodes and bad sys data", twoNodes, badSysData, SLIT{
			[]Cost{0, 0},
			[]Cost{0, 0},
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

func TestSysfs_discoverCores(t *testing.T) {
	st := NewTopology(idset.Empty[hw.NodeID](), SLIT{}, []Core{})
	// twoNodes := idset.From[hw.NodeID]([]uint8{1, 3})

	tests := []struct {
		name             string
		onlineCores      *idset.Set[hw.CoreID]
		nodeIDs          *idset.Set[hw.NodeID]
		readerFunc       pathReaderFn
		expectedTopology *Topology
	}{
		{"empty node IDs", idset.Empty[hw.CoreID](), idset.Empty[hw.NodeID](), os.ReadFile, &Topology{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sy := &Sysfs{}
			st.NodeIDs = tt.nodeIDs
			sy.discoverCores(st, tt.readerFunc)
			must.Eq(t, tt.expectedTopology, st)
		})
	}
}
