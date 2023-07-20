package numalib

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/lib/idset"
)

const (
	sysRoot      = "/sys/devices/system"
	nodeOnline   = sysRoot + "/node/online"
	distanceFile = sysRoot + "/node/node%d/distance"
	cpulistFile  = sysRoot + "/node/node%d/cpulist"
)

func ScanSysfs() *Topology {
	st := new(Topology)

	// detect the online numa nodes
	discoverOnline(st)

	// detect cross numa node latency costs
	discoverCosts(st)

	// detect cores
	discoverCores(st)

	return st
}

var (
	distanceRe = regexp.MustCompile(`^/sys/devices/system/node/node([\d]+)/distance$`)
)

func discoverOnline(st *Topology) {
	s, err := os.ReadFile(nodeOnline)
	if err != nil {
		return
	}

	ids := idset.Parse[nodeID](string(s))
	st.nodes = ids
}

func discoverCosts(st *Topology) {
	dimension := st.nodes.Size()
	st.distances = make(distances, st.nodes.Size())
	for i := 0; i < dimension; i++ {
		st.distances[i] = make([]Latency, dimension)
	}

	_ = st.nodes.ForEach(func(id nodeID) error {
		s, err := os.ReadFile(fmt.Sprintf(distanceFile, id))
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
	_ = st.nodes.ForEach(func(id nodeID) error {
		s, err := os.ReadFile(fmt.Sprintf(cpulistFile, id))
		if err != nil {
			return err
		}

		ids := idset.Parse[coreID](string(s))
		fmt.Println("node", id, "core ids", ids)
		return nil
	})
}
