// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package numalib

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/idset"
)

// PlatformScanners returns the set of SystemScanner for Linux.
func PlatformScanners() []SystemScanner {
	return []SystemScanner{
		new(Sysfs),
		new(Smbios),
		new(Cgroups1),
		new(Cgroups2),
	}
}

const (
	sysRoot        = "/sys/devices/system"
	nodeOnline     = sysRoot + "/node/online"
	cpuOnline      = sysRoot + "/cpu/online"
	distanceFile   = sysRoot + "/node/node%d/distance"
	cpulistFile    = sysRoot + "/node/node%d/cpulist"
	cpuMaxFile     = sysRoot + "/cpu/cpu%d/cpufreq/cpuinfo_max_freq"
	cpuBaseFile    = sysRoot + "/cpu/cpu%d/cpufreq/base_frequency"
	cpuSocketFile  = sysRoot + "/cpu/cpu%d/topology/physical_package_id"
	cpuSiblingFile = sysRoot + "/cpu/cpu%d/topology/thread_siblings_list"
)

// Sysfs implements SystemScanner for Linux by reading system topology data
// from /sys/devices/system. This is the best source of truth on Linux and
// should always be used first - additional scanners can provide more context
// on top of what is initiallly detected here.
type Sysfs struct{}

func (s *Sysfs) ScanSystem(top *Topology) {
	// detect the online numa nodes
	s.discoverOnline(top)

	// detect cross numa node latency costs
	s.discoverCosts(top)

	// detect core performance data
	s.discoverCores(top)
}

func (*Sysfs) available() bool {
	return true
}

func (*Sysfs) discoverOnline(st *Topology) {
	ids, err := getIDSet[NodeID](nodeOnline)
	if err == nil {
		st.NodeIDs = ids
	}
}

func (*Sysfs) discoverCosts(st *Topology) {
	dimension := st.NodeIDs.Size()
	st.Distances = make(SLIT, st.NodeIDs.Size())
	for i := 0; i < dimension; i++ {
		st.Distances[i] = make([]Cost, dimension)
	}

	_ = st.NodeIDs.ForEach(func(id NodeID) error {
		s, err := getString(distanceFile, id)
		if err != nil {
			return err
		}

		for i, c := range strings.Fields(s) {
			cost, _ := strconv.Atoi(c)
			st.Distances[id][i] = Cost(cost)
		}
		return nil
	})
}

func (*Sysfs) discoverCores(st *Topology) {
	onlineCores, err := getIDSet[CoreID](cpuOnline)
	if err != nil {
		return
	}
	st.Cores = make([]Core, onlineCores.Size())

	_ = st.NodeIDs.ForEach(func(node NodeID) error {
		s, err := os.ReadFile(fmt.Sprintf(cpulistFile, node))
		if err != nil {
			return err
		}

		cores := idset.Parse[CoreID](string(s))
		_ = cores.ForEach(func(core CoreID) error {
			// best effort, zero values are defaults
			socket, _ := getNumeric[SocketID](cpuSocketFile, core)
			max, _ := getNumeric[KHz](cpuMaxFile, core)
			base, _ := getNumeric[KHz](cpuBaseFile, core)
			siblings, _ := getIDSet[CoreID](cpuSiblingFile, core)
			st.insert(node, socket, core, gradeOf(siblings), max, base)
			return nil
		})
		return nil
	})
}

func getIDSet[T idset.ID](path string, args ...any) (*idset.Set[T], error) {
	path = fmt.Sprintf(path, args...)
	s, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return idset.Parse[T](string(s)), nil
}

func getNumeric[T int | idset.ID](path string, args ...any) (T, error) {
	path = fmt.Sprintf(path, args...)
	s, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(strings.TrimSpace(string(s)))
	if err != nil {
		return 0, err
	}
	return T(i), nil
}

func getString(path string, args ...any) (string, error) {
	path = fmt.Sprintf(path, args...)
	s, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(s)), nil
}

// Cgroups1 reads effective cores information from cgroups v1
type Cgroups1 struct{}

func (s *Cgroups1) ScanSystem(top *Topology) {
	if cgroupslib.GetMode() != cgroupslib.CG1 {
		return
	}

	// detect effective cores in the cpuset/nomad cgroup
	content, err := cgroupslib.ReadNomadCG1("cpuset", "cpuset.effective_cpus")
	if err != nil {
		return
	}

	// extract IDs from file of ids
	scanIDs(top, content)
}

// Cgroups2 reads effective cores information from cgroups v2
type Cgroups2 struct{}

func (s *Cgroups2) ScanSystem(top *Topology) {
	if cgroupslib.GetMode() != cgroupslib.CG2 {
		return
	}

	// detect effective cores in the nomad.slice cgroup
	content, err := cgroupslib.ReadNomadCG2("cpuset.cpus.effective")
	if err != nil {
		return
	}

	// extract IDs from file of ids
	scanIDs(top, content)
}

// combine scanCgroups
func scanIDs(top *Topology, content string) {
	ids := idset.Parse[CoreID](content)
	for _, cpu := range top.Cores {
		if !ids.Contains(cpu.ID) {
			cpu.Disable = true
		}
	}
}
