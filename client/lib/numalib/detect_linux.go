//go:build linux

package numalib

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
)

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

type Sysfs struct{}

func (s *Sysfs) ScanSystem(top *Topology) {
	// detect the online numa nodes
	discoverOnline(top)

	// detect cross numa node latency costs
	discoverCosts(top)

	// detect core performance data
	discoverCores(top)
}

func discoverOnline(st *Topology) {
	ids, err := getIDSet[NodeID](nodeOnline)
	if err == nil {
		st.NodeIDs = ids
	}
}

func discoverCosts(st *Topology) {
	dimension := st.NodeIDs.Size()
	st.Distances = make(Distances, st.NodeIDs.Size())
	for i := 0; i < dimension; i++ {
		st.Distances[i] = make([]Latency, dimension)
	}

	_ = st.NodeIDs.ForEach(func(id NodeID) error {
		s, err := getString(distanceFile, id)
		if err != nil {
			return err
		}

		for i, c := range strings.Fields(string(s)) {
			cost, _ := strconv.Atoi(c)
			st.Distances[id][i] = Latency(cost)
		}
		return nil
	})
}

func discoverCores(st *Topology) {
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
			socket, err := getNumeric[SocketID](cpuSocketFile, core)
			if err != nil {
				return err
			}

			max, err := getNumeric[KHz](cpuMaxFile, core)
			if err != nil {
				return err
			}

			base, _ := getNumeric[KHz](cpuBaseFile, core)
			// not set on many systems

			siblings, err := getIDSet[CoreID](cpuSiblingFile, core)
			if err != nil {
				return err
			}

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

// SETH TODO:
// fallbacks (e.g. dmi decode)
// better error handling?

// cgroups

type Cgroups1 struct{}

func (s *Cgroups1) ScanSystem(top *Topology) {
	if cgroupslib.GetMode() != cgroupslib.CG1 {
		return
	}

	// detect effective cores in the cpuset/nomad cgroup
	content, err := cgroupslib.ReadNomadCG1("cpuset", "cpuset.effective_cpus")
	if err == nil {
		ids := idset.Parse[CoreID](content)
		for _, cpu := range top.Cores {
			if !ids.Contains(cpu.ID) {
				cpu.Disable = true
			}
		}
	}
}

type Cgroups2 struct{}

func (s *Cgroups2) ScanSystem(top *Topology) {
	if cgroupslib.GetMode() != cgroupslib.CG2 {
		return
	}

	// detect effective cores in the nomad.slice cgroup
	content, err := cgroupslib.ReadNomadCG2("cpuset.cpus.effective")
	if err == nil {
		ids := idset.Parse[CoreID](content)
		for _, cpu := range top.Cores {
			if !ids.Contains(cpu.ID) {
				cpu.Disable = true
			}
		}
	}
}

// combine scanCgroups
