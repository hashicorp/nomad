package cpuset

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// CPUSet is a set like object that provides methods helpful when working with cpus with systems
// such as the Linux cpuset cgroup subsystem. A CPUSet is immutable and can be safely accessed concurrently.
type CPUSet struct {
	cpus map[uint16]struct{}
}

// New initializes a new CPUSet with 0 or more containing cpus
func New(cpus ...uint16) CPUSet {
	cpuset := CPUSet{
		cpus: make(map[uint16]struct{}),
	}

	for _, v := range cpus {
		cpuset.cpus[v] = struct{}{}
	}

	return cpuset
}

// Size returns to the number of cpus contained in the CPUSet
func (c CPUSet) Size() int {
	return len(c.cpus)
}

// ToSlice returns a sorted slice of uint16 CPU IDs contained in the CPUSet.
func (c CPUSet) ToSlice() []uint16 {
	cpus := []uint16{}
	for k := range c.cpus {
		cpus = append(cpus, k)
	}
	sort.Slice(cpus, func(i, j int) bool { return cpus[i] < cpus[j] })
	return cpus
}

// Union returns a new set that is the union of this CPUSet and the supplied other.
// Ex. [0,1,2,3].Union([2,3,4,5]) = [0,1,2,3,4,5]
func (c CPUSet) Union(other CPUSet) CPUSet {
	s := New()
	for k := range c.cpus {
		s.cpus[k] = struct{}{}
	}
	for k := range other.cpus {
		s.cpus[k] = struct{}{}
	}
	return s
}

// Difference returns a new set that is the difference of this CPUSet and the supplied other.
// [0,1,2,3].Difference([2,3,4]) = [0,1]
func (c CPUSet) Difference(other CPUSet) CPUSet {
	s := New()
	for k := range c.cpus {
		s.cpus[k] = struct{}{}
	}
	for k := range other.cpus {
		delete(s.cpus, k)
	}
	return s

}

// IsSubsetOf returns true if all cpus of the this CPUSet are present in the other CPUSet.
func (s CPUSet) IsSubsetOf(other CPUSet) bool {
	for cpu := range s.cpus {
		if _, ok := other.cpus[cpu]; !ok {
			return false
		}
	}
	return true
}

func (s CPUSet) IsSupersetOf(other CPUSet) bool {
	for cpu := range other.cpus {
		if _, ok := s.cpus[cpu]; !ok {
			return false
		}
	}
	return true
}

// Equals tests the equality of the elements in the CPUSet
func (s CPUSet) Equals(other CPUSet) bool {
	return reflect.DeepEqual(s.cpus, other.cpus)
}

// Parse parses the Linux cpuset format into a CPUSet
//
// Ref: http://man7.org/linux/man-pages/man7/cpuset.7.html#FORMATS
func Parse(s string) (CPUSet, error) {
	cpuset := New()
	if s == "" {
		return cpuset, nil
	}
	sets := strings.Split(s, ",")
	for _, set := range sets {
		bounds := strings.Split(set, "-")
		if len(bounds) == 1 {
			v, err := strconv.Atoi(bounds[0])
			if err != nil {
				return New(), err
			}

			cpuset.cpus[uint16(v)] = struct{}{}
			continue
		}
		if len(bounds) > 2 {
			return New(), fmt.Errorf("failed to parse element %s, more than 1 '-' found", set)
		}

		lower, err := strconv.Atoi(bounds[0])
		if err != nil {
			return New(), err
		}
		upper, err := strconv.Atoi(bounds[1])
		if err != nil {
			return New(), err
		}
		for v := lower; v <= upper; v++ {
			cpuset.cpus[uint16(v)] = struct{}{}
		}
	}

	return cpuset, nil
}
