// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package numalib

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// PlatformScanners returns the set of SystemScanner for Linux.
func PlatformScanners() []SystemScanner {
	return []SystemScanner{
		new(Sysfs),
		new(Smbios),
		new(Cpuinfo),
		new(Cgroups1),
		new(Cgroups2),
		new(Fallback),
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

// pathReaderFn is a path reader function, injected into all value getters to
// ease testing.
type pathReaderFn func(string) ([]byte, error)

// Sysfs implements SystemScanner for Linux by reading system topology data
// from /sys/devices/system. This is the best source of truth on Linux and
// should always be used first - additional scanners can provide more context
// on top of what is initiallly detected here.
//
// Any data from Sysfs takes priority over Smbios.
// Any data from Smbios takes priority over Cpuinfo.
type Sysfs struct{}

func (s *Sysfs) ScanSystem(top *Topology) {
	// detect the online numa nodes
	s.discoverOnline(top, os.ReadFile)

	// detect cross numa node latency costs
	s.discoverCosts(top, os.ReadFile)

	// detect core performance data
	s.discoverCores(top, os.ReadFile)
}

func (*Sysfs) available() bool {
	return true
}

func (*Sysfs) discoverOnline(st *Topology, readerFunc pathReaderFn) {
	ids, err := getIDSet[hw.NodeID](nodeOnline, readerFunc)
	if err == nil {
		st.NodeIDs = ids
	}
}

func (*Sysfs) discoverCosts(st *Topology, readerFunc pathReaderFn) {
	if st.NodeIDs.Empty() {
		return
	}

	dimension := st.NodeIDs.Size()
	st.Distances = make(SLIT, st.NodeIDs.Size())
	for i := 0; i < dimension; i++ {
		st.Distances[i] = make([]Cost, dimension)
	}

	_ = st.NodeIDs.ForEach(func(id hw.NodeID) error {
		s, err := getString(distanceFile, readerFunc, id)
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

func (*Sysfs) discoverCores(st *Topology, readerFunc pathReaderFn) {
	onlineCores, err := getIDSet[hw.CoreID](cpuOnline, readerFunc)
	if err != nil {
		return
	}
	st.Cores = make([]Core, onlineCores.Size())

	switch {
	case st.NodeIDs == nil:
		// We did not find node data, no node to associate with
		_ = onlineCores.ForEach(func(core hw.CoreID) error {
			st.NodeIDs = idset.From[hw.NodeID]([]hw.NodeID{0})
			const node = 0
			const socket = 0
			cpuMax, _ := getNumeric[hw.KHz](cpuMaxFile, readerFunc, core)
			base, _ := getNumeric[hw.KHz](cpuBaseFile, readerFunc, core)
			st.insert(node, socket, core, Performance, cpuMax, base)
			return nil
		})
	default:
		// We found node data, associate cores to nodes
		_ = st.NodeIDs.ForEach(func(node hw.NodeID) error {
			s, err := readerFunc(fmt.Sprintf(cpulistFile, node))
			if err != nil {
				return err
			}

			cores := idset.Parse[hw.CoreID](string(s))
			_ = cores.ForEach(func(core hw.CoreID) error {
				// best effort, zero values are defaults
				socket, _ := getNumeric[hw.SocketID](cpuSocketFile, readerFunc, core)
				cpuMax, _ := getNumeric[hw.KHz](cpuMaxFile, readerFunc, core)
				base, _ := getNumeric[hw.KHz](cpuBaseFile, readerFunc, core)
				siblings, _ := getIDSet[hw.CoreID](cpuSiblingFile, readerFunc, core)

				// if we get an incorrect core number, this means we're not getting the right
				// data from SysFS. In this case we bail and set default values.
				if int(core) >= len(st.Cores) {
					return nil
				}

				st.insert(node, socket, core, gradeOf(siblings), cpuMax, base)
				return nil
			})
			return nil
		})
	}
}

func getIDSet[T idset.ID](path string, readerFunc pathReaderFn, args ...any) (*idset.Set[T], error) {
	path = fmt.Sprintf(path, args...)
	s, err := readerFunc(path)
	if err != nil {
		return nil, err
	}
	return idset.Parse[T](string(s)), nil
}

func getNumeric[T int | idset.ID](path string, readerFunc pathReaderFn, args ...any) (T, error) {
	path = fmt.Sprintf(path, args...)
	s, err := readerFunc(path)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(strings.TrimSpace(string(s)))
	if err != nil {
		return 0, err
	}
	return T(i), nil
}

func getString(path string, readerFunc pathReaderFn, args ...any) (string, error) {
	path = fmt.Sprintf(path, args...)
	s, err := readerFunc(path)
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
	ids := idset.Parse[hw.CoreID](content)
	for _, cpu := range top.Cores {
		if !ids.Contains(cpu.ID) {
			cpu.Disable = true
		}
	}
}

// Cpuinfo implements SystemScanner for Linux by reading CPU speeds out of
// /proc/cpuinfo. Note that these values are unreliable and represent the current
// CPU speeds, not the base or max speed. We take the average speed of all cores
// and set that as the guess speed for all cores, which is hopefully in the
// ballpark of the true base speed.
type Cpuinfo struct {
	cpuinfo string
}

func (s *Cpuinfo) ScanSystem(top *Topology) {
	if s.cpuinfo == "" {
		// the real special file
		s.cpuinfo = "/proc/cpuinfo"
	}

	f, err := os.Open(s.cpuinfo)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	s.discoverSpeeds(top, f)
}

var (
	// cpu MHz     : 2000.000
	cpuinfoRe = regexp.MustCompile(`cpu MHz\s+:\s+(\d+)`)
)

func (s *Cpuinfo) discoverSpeeds(top *Topology, reader io.Reader) {
	count := 0
	sum := hw.MHz(0)

	// scan lines of /proc/cpuinfo looking for current MHz values, so we can
	// tally them up and take an average
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		matches := cpuinfoRe.FindStringSubmatch(line)
		if len(matches) == 2 {
			if value, err := strconv.Atoi(matches[1]); err == nil {
				sum += hw.MHz(value)
				count += 1
			}
		}
	}
	if scanner.Err() != nil {
		// if the scanner failed just bail
		return
	}

	if count == 0 {
		// if we found no values just bail
		return
	}

	// compute the average speed
	avg := sum / hw.MHz(count)

	// set the guess speed for each core but only if the value has not yet
	// been set by other means
	for i := 0; i < len(top.Cores); i++ {
		if top.Cores[i].GuessSpeed == 0 {
			top.Cores[i].GuessSpeed = avg
		}
	}
}

// Fallback detects if the NUMA aware topology scanning was unable to construct
// a valid model of the system. This will be common on Nomad clients running in
// containers, erroneous hypervisors, or without root.
type Fallback struct{}

func (s *Fallback) ScanSystem(top *Topology) {
	broken := false

	switch {
	case top.NodeIDs.Empty():
		broken = true
	case len(top.Distances) == 0:
		broken = true
	case top.NumCores() <= 0:
		broken = true
	case top.TotalCompute() <= 0:
		broken = true
	case top.UsableCompute() <= 0:
		broken = true
	case top.UsableCores().Empty():
		broken = true
	}

	if !broken {
		return
	}

	// we have a broken topology; reset it and fallback to the generic scanner
	// basically treating this client like a windows / unsupported OS
	top.NodeIDs = nil
	top.Distances = nil
	top.Cores = nil

	// invoke the generic scanner
	scanGeneric(top)
}
