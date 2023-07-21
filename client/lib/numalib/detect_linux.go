package numalib

import (
	"github.com/shoenig/netlog"

	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/lib/idset"
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
	ids, err := getIDSet[nodeID](nodeOnline)
	if err == nil {
		st.nodes = ids
	}
}

func discoverCosts(st *Topology) {
	dimension := st.nodes.Size()
	st.distances = make(distances, st.nodes.Size())
	for i := 0; i < dimension; i++ {
		st.distances[i] = make([]Latency, dimension)
	}

	_ = st.nodes.ForEach(func(id nodeID) error {
		s, err := getString(distanceFile, id)
		if err != nil {
			return err
		}

		for i, c := range strings.Fields(string(s)) {
			cost, _ := strconv.Atoi(c)
			st.distances[id][i] = Latency(cost)
		}
		return nil
	})
}

func discoverCores(st *Topology) {
	onlineCores, err := getIDSet[CoreID](cpuOnline)
	if err != nil {
		return
	}
	st.cpus = make([]Core, onlineCores.Size())

	_ = st.nodes.ForEach(func(node nodeID) error {
		s, err := os.ReadFile(fmt.Sprintf(cpulistFile, node))
		if err != nil {
			return err
		}

		cores := idset.Parse[CoreID](string(s))
		fmt.Println("node", node, "core ids", cores)
		_ = cores.ForEach(func(core CoreID) error {
			socket, err := getNumeric[socketID](cpuSocketFile, core)
			if err != nil {
				fmt.Println("err", err)
				return err
			}

			max, err := getNumeric[KHz](cpuMaxFile, core)
			if err != nil {
				fmt.Println("err", err)
				return err
			}

			base, _ := getNumeric[KHz](cpuBaseFile, core)
			// not set on many systems

			netlog.Cyan("cpu", "max", uint64(max), "mhz", max, "base", base)

			siblings, err := getIDSet[CoreID](cpuSiblingFile, core)
			if err != nil {
				fmt.Println("err", err)
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

// YOU ARE HERE
// fallbacks ?
// better error handling?
