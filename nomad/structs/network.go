package structs

import (
	"fmt"
	"net"
	"sync"
)

const (
	// MinDynamicPort is the smallest dynamic port generated
	MinDynamicPort = 20000

	// MaxDynamicPort is the largest dynamic port generated
	MaxDynamicPort = 60000

	// maxRandPortAttempts is the maximum number of attempt
	// to assign a random port
	maxRandPortAttempts = 20

	// maxValidPort is the max valid port number
	maxValidPort = 65536
)

var (
	// bitmapPool is used to pool the bitmaps used for port collision
	// checking. They are fairly large (8K) so we can re-use them to
	// avoid GC pressure. Care should be taken to call Clear() on any
	// bitmap coming from the pool.
	bitmapPool = new(sync.Pool)
)

// NetworkIndex is used to index the available network resources
// and the used network resources on a machine given allocations
type NetworkIndex struct {
	AvailNetworks  []*NetworkResource // List of available networks
	AvailBandwidth map[string]int     // Bandwidth by device
	UsedPorts      map[string]Bitmap  // Ports by IP
	UsedBandwidth  map[string]int     // Bandwidth by device
}

// NewNetworkIndex is used to construct a new network index
func NewNetworkIndex() *NetworkIndex {
	return &NetworkIndex{
		AvailBandwidth: make(map[string]int),
		UsedPorts:      make(map[string]Bitmap),
		UsedBandwidth:  make(map[string]int),
	}
}

// Release is called when the network index is no longer needed
// to attempt to re-use some of the memory it has allocated
func (idx *NetworkIndex) Release() {
	for _, b := range idx.UsedPorts {
		bitmapPool.Put(b)
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
		if n.Device != "" {
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
		// Try to get a bitmap from the pool, else create
		raw := bitmapPool.Get()
		if raw != nil {
			used = raw.(Bitmap)
			used.Clear()
		} else {
			used, _ = NewBitmap(maxValidPort)
		}
		idx.UsedPorts[n.IP] = used
	}

	for _, ports := range [][]Port{n.ReservedPorts, n.DynamicPorts} {
		for _, port := range ports {
			// Guard against invalid port
			if port.Value < 0 || port.Value >= maxValidPort {
				return true
			}
			if used.Check(uint(port.Value)) {
				collide = true
			} else {
				used.Set(uint(port.Value))
			}
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
	err = fmt.Errorf("no networks available")
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

		used := idx.UsedPorts[ipStr]

		// Check if any of the reserved ports are in use
		for _, port := range ask.ReservedPorts {
			// Guard against invalid port
			if port.Value < 0 || port.Value >= maxValidPort {
				err = fmt.Errorf("invalid port %d (out of range)", port.Value)
				return
			}

			// Check if in use
			if used != nil && used.Check(uint(port.Value)) {
				err = fmt.Errorf("reserved port collision")
				return
			}
		}

		// Create the offer
		offer := &NetworkResource{
			Device:        n.Device,
			IP:            ipStr,
			MBits:         ask.MBits,
			ReservedPorts: ask.ReservedPorts,
			DynamicPorts:  ask.DynamicPorts,
		}

		portRange := PortsFromRange(MinDynamicPort, MaxDynamicPort)
		usedPorts := PortsFromBitmap(used, 0, maxValidPort-1)
		availablePorts := portRange.Difference(usedPorts)
		availablePorts = availablePorts.Difference(ask.ReservedPorts)
		availablePorts = availablePorts.ShufflePorts()

		if len(availablePorts) < len(offer.DynamicPorts) {
			err = fmt.Errorf("dynamic port selection failed - insufficient available ports")
			return
		}

		for i := 0; i < len(offer.DynamicPorts); i++ {
			offer.DynamicPorts[i].Value = availablePorts[i].Value
		}

		// Stop, we have an offer!
		out = offer
		err = nil
		return true
	})
	return
}

func isPortReserved(haystack []Port, needle int) bool {
	for _, item := range haystack {
		if item.Value == needle {
			return true
		}
	}
	return false
}
