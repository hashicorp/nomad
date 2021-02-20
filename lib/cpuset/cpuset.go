package cpuset

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type CPUSet struct {
	cpus map[uint32]struct{}
}

func NewCPUSet() CPUSet {
	return CPUSet{
		cpus: make(map[uint32]struct{}),
	}
}

func (c CPUSet) Size() int {
	return len(c.cpus)
}

func (c CPUSet) ToSlice() []uint32 {
	cpus := []uint32{}
	for k := range c.cpus {
		cpus = append(cpus, k)
	}
	sort.Slice(cpus, func(i, j int) bool { return cpus[i] < cpus[j] })
	return cpus
}

func (c CPUSet) Union(other CPUSet) CPUSet {
	s := NewCPUSet()
	for k := range c.cpus {
		s.cpus[k] = struct{}{}
	}
	for k := range other.cpus {
		s.cpus[k] = struct{}{}
	}
	return s
}

func (c CPUSet) Difference(other CPUSet) CPUSet {
	s := NewCPUSet()
	for k := range c.cpus {
		s.cpus[k] = struct{}{}
	}
	for k := range other.cpus {
		delete(s.cpus, k)
	}
	return s

}

func (s CPUSet) IsSubsetOf(other CPUSet) bool {
	for cpu := range s.cpus {
		if _, ok := other.cpus[cpu]; !ok {
			return false
		}
	}
	return true
}

func FromSlice(s []uint32) CPUSet {
	cpuset := NewCPUSet()
	for _, v := range s {
		cpuset.cpus[v] = struct{}{}
	}
	return cpuset
}

func Parse(s string) (CPUSet, error) {
	cpuset := NewCPUSet()
	sets := strings.Split(s, ",")
	for _, set := range sets {
		bounds := strings.Split(set, "-")
		if len(bounds) == 1 {
			v, err := strconv.Atoi(bounds[0])
			if err != nil {
				return NewCPUSet(), err
			}

			cpuset.cpus[uint32(v)] = struct{}{}
		}
		if len(bounds) > 2 {
			return NewCPUSet(), fmt.Errorf("failed to parse element %s, more than 1 '-' found", set)
		}

		lower, err := strconv.Atoi(bounds[0])
		if err != nil {
			return NewCPUSet(), err
		}
		upper, err := strconv.Atoi(bounds[1])
		if err != nil {
			return NewCPUSet(), err
		}
		for v := lower; v <= upper; v++ {
			cpuset.cpus[uint32(v)] = struct{}{}
		}
	}

	return cpuset, nil
}
