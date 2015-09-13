package structs

import (
	"fmt"
	"math/rand"
	"net"
)

const (
	// MinDynamicPort is the smallest dynamic port generated
	MinDynamicPort = 20000

	// MaxDynamicPort is the largest dynamic port generated
	MaxDynamicPort = 60000

	// maxRandPortAttempts is the maximum number of attempt
	// to assign a random port
	maxRandPortAttempts = 20
)

// NetworkIndex is used to index the available network resources
// and the used network resources on a machine given allocations
type NetworkIndex struct {
	AvailNetworks  []*NetworkResource          // List of available networks
	AvailBandwidth map[string]int              // Bandwidth by device
	UsedPorts      map[string]map[int]struct{} // Ports by IP
	UsedBandwidth  map[string]int              // Bandwidth by device
}

// NewNetworkIndex is used to construct a new network index
func NewNetworkIndex() *NetworkIndex {
	return &NetworkIndex{
		AvailBandwidth: make(map[string]int),
		UsedPorts:      make(map[string]map[int]struct{}),
		UsedBandwidth:  make(map[string]int),
	}
}

// Overcommitted checks if the network is overcommitted
func (idx *NetworkIndex) Overcommitted() bool {
	for device, used := range idx.UsedBandwidth {
		avail := idx.AvailBandwidth[device]
		if used > avail {
			return true
		}
	}
	return false
}

// SetNode is used to setup the available network resources. Returns
// true if there is a collision
func (idx *NetworkIndex) SetNode(node *Node) (collide bool) {
	// Add the available CIDR blocks
	for _, n := range node.Resources.Networks {
		if n.CIDR != "" {
			idx.AvailNetworks = append(idx.AvailNetworks, n)
			idx.AvailBandwidth[n.Device] = n.MBits
		}
	}

	// Add the reserved resources
	if r := node.Reserved; r != nil {
		for _, n := range r.Networks {
			if idx.AddReserved(n) {
				collide = true
			}
		}
	}
	return
}

// AddAllocs is used to add the used network resources. Returns
// true if there is a collision
func (idx *NetworkIndex) AddAllocs(allocs []*Allocation) (collide bool) {
	for _, alloc := range allocs {
		for _, task := range alloc.TaskResources {
			if len(task.Networks) == 0 {
				continue
			}
			n := task.Networks[0]
			if idx.AddReserved(n) {
				collide = true
			}
		}
	}
	return
}

// AddReserved is used to add a reserved network usage, returns true
// if there is a port collision
func (idx *NetworkIndex) AddReserved(n *NetworkResource) (collide bool) {
	// Add the port usage
	used := idx.UsedPorts[n.IP]
	if used == nil {
		used = make(map[int]struct{})
		idx.UsedPorts[n.IP] = used
	}
	for _, port := range n.ReservedPorts {
		if _, ok := used[port]; ok {
			collide = true
		} else {
			used[port] = struct{}{}
		}
	}

	// Add the bandwidth
	idx.UsedBandwidth[n.Device] += n.MBits
	return
}

// yieldIP is used to iteratively invoke the callback with
// an available IP
func (idx *NetworkIndex) yieldIP(cb func(net *NetworkResource, ip net.IP) bool) {
	inc := func(ip net.IP) {
		for j := len(ip) - 1; j >= 0; j-- {
			ip[j]++
			if ip[j] > 0 {
				break
			}
		}
	}

	for _, n := range idx.AvailNetworks {
		ip, ipnet, err := net.ParseCIDR(n.CIDR)
		if err != nil {
			continue
		}
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			if cb(n, ip) {
				return
			}
		}
	}
}

// AssignNetwork is used to assign network resources given an ask.
// If the ask cannot be satisfied, returns nil
func (idx *NetworkIndex) AssignNetwork(ask *NetworkResource) (out *NetworkResource, err error) {
	idx.yieldIP(func(n *NetworkResource, ip net.IP) (stop bool) {
		// Convert the IP to a string
		ipStr := ip.String()

		// Check if we would exceed the bandwidth cap
		availBandwidth := idx.AvailBandwidth[n.Device]
		usedBandwidth := idx.UsedBandwidth[n.Device]
		if usedBandwidth+ask.MBits > availBandwidth {
			err = fmt.Errorf("bandwidth exceeded")
			return
		}

		// Check if any of the reserved ports are in use
		for _, port := range ask.ReservedPorts {
			if _, ok := idx.UsedPorts[ipStr][port]; ok {
				err = fmt.Errorf("reserved port collision")
				return
			}
		}

		// Create the offer
		offer := &NetworkResource{
			Device:        n.Device,
			IP:            ipStr,
			ReservedPorts: ask.ReservedPorts,
		}

		// Check if we need to generate any ports
		for i := 0; i < ask.DynamicPorts; i++ {
			attempts := 0
		PICK:
			attempts++
			if attempts > maxRandPortAttempts {
				err = fmt.Errorf("dynamic port selection failed")
				return
			}

			randPort := MinDynamicPort + rand.Intn(MaxDynamicPort-MinDynamicPort)
			if _, ok := idx.UsedPorts[ipStr][randPort]; ok {
				goto PICK
			}
			if IntContains(offer.ReservedPorts, randPort) {
				goto PICK
			}
			offer.ReservedPorts = append(offer.ReservedPorts, randPort)
		}

		// Stop, we have an offer!
		out = offer
		err = nil
		return true
	})
	return
}

// IntContains scans an integer slice for a value
func IntContains(haystack []int, needle int) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
