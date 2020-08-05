package structs

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
)

// PortRange describes the boundaries of a contiguous port range
type PortRange struct {
	Min int
	Max int
}

var (
	// default values
	dynamicPortRangeMin = 20000
	dynamicPortRangeMax = 32000

	// globally configurable dynamic port range
	dynamicPortRangeLock sync.Mutex
	dynamicPortRange     = PortRange{
		Min: dynamicPortRangeMin,
		Max: dynamicPortRangeMax,
	}
)

// GetDynamicPortRange returns the globally defined dynamic port range (default: 20000-32000).
func GetDynamicPortRange() PortRange {
	return dynamicPortRange
}

// SetDynamicPortRange reconfigures the dynamic port range.
func SetDynamicPortRange(p PortRange) {
	dynamicPortRangeLock.Lock()
	defer dynamicPortRangeLock.Unlock()
	dynamicPortRange = p
}

const (
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
	AvailNetworks  []*NetworkResource              // List of available networks
	NodeNetworks   []*NodeNetworkResource          // List of available node networks
	AvailAddresses map[string][]NodeNetworkAddress // Map of host network aliases to list of addresses
	AvailBandwidth map[string]int                  // Bandwidth by device
	UsedPorts      map[string]Bitmap               // Ports by IP
	UsedBandwidth  map[string]int                  // Bandwidth by device
}

// NewNetworkIndex is used to construct a new network index
func NewNetworkIndex() *NetworkIndex {
	return &NetworkIndex{
		AvailAddresses: make(map[string][]NodeNetworkAddress),
		AvailBandwidth: make(map[string]int),
		UsedPorts:      make(map[string]Bitmap),
		UsedBandwidth:  make(map[string]int),
	}
}

func (idx *NetworkIndex) getUsedPortsFor(ip string) Bitmap {
	used := idx.UsedPorts[ip]
	if used == nil {
		// Try to get a bitmap from the pool, else create
		raw := bitmapPool.Get()
		if raw != nil {
			used = raw.(Bitmap)
			used.Clear()
		} else {
			used, _ = NewBitmap(maxValidPort)
		}
		idx.UsedPorts[ip] = used
	}
	return used
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
	// TODO remove since bandwidth is deprecated
	/*for device, used := range idx.UsedBandwidth {
		avail := idx.AvailBandwidth[device]
		if used > avail {
			return true
		}
	}*/
	return false
}

// SetNode is used to setup the available network resources. Returns
// true if there is a collision
func (idx *NetworkIndex) SetNode(node *Node) (collide bool) {

	// COMPAT(0.11): Remove in 0.11
	// Grab the network resources, handling both new and old
	var networks []*NetworkResource
	if node.NodeResources != nil && len(node.NodeResources.Networks) != 0 {
		networks = node.NodeResources.Networks
	} else if node.Resources != nil {
		networks = node.Resources.Networks
	}

	var nodeNetworks []*NodeNetworkResource
	if node.NodeResources != nil && len(node.NodeResources.NodeNetworks) != 0 {
		nodeNetworks = node.NodeResources.NodeNetworks
	}

	// Add the available CIDR blocks
	for _, n := range networks {
		if n.Device != "" {
			idx.AvailNetworks = append(idx.AvailNetworks, n)
			idx.AvailBandwidth[n.Device] = n.MBits
		}
	}

	// TODO: upgrade path?
	// is it possible to get duplicates here?
	for _, n := range nodeNetworks {
		for _, a := range n.Addresses {
			idx.AvailAddresses[a.Alias] = append(idx.AvailAddresses[a.Alias], a)
			if idx.AddReservedPortsForIP(a.ReservedPorts, a.Address) {
				collide = true
			}
		}
	}

	// COMPAT(0.11): Remove in 0.11
	// Handle reserving ports, handling both new and old
	if node.ReservedResources != nil && node.ReservedResources.Networks.ReservedHostPorts != "" {
		collide = idx.AddReservedPortRange(node.ReservedResources.Networks.ReservedHostPorts)
	} else if node.Reserved != nil {
		for _, n := range node.Reserved.Networks {
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
		// Do not consider the resource impact of terminal allocations
		if alloc.TerminalStatus() {
			continue
		}

		if alloc.AllocatedResources != nil {
			// Only look at AllocatedPorts if populated, otherwise use pre 0.12 logic
			// COMPAT(1.0): Remove when network resources struct is removed.
			if len(alloc.AllocatedResources.Shared.Ports) > 0 {
				if idx.AddReservedPorts(alloc.AllocatedResources.Shared.Ports) {
					collide = true
				}
			} else {
				// Add network resources that are at the task group level
				if len(alloc.AllocatedResources.Shared.Networks) > 0 {
					for _, network := range alloc.AllocatedResources.Shared.Networks {
						if idx.AddReserved(network) {
							collide = true
						}
					}
				}

				for _, task := range alloc.AllocatedResources.Tasks {
					if len(task.Networks) == 0 {
						continue
					}
					n := task.Networks[0]
					if idx.AddReserved(n) {
						collide = true
					}
				}
			}
		} else {
			// COMPAT(0.11): Remove in 0.11
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
	}
	return
}

// AddReserved is used to add a reserved network usage, returns true
// if there is a port collision
func (idx *NetworkIndex) AddReserved(n *NetworkResource) (collide bool) {
	// Add the port usage
	used := idx.getUsedPortsFor(n.IP)

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

func (idx *NetworkIndex) AddReservedPorts(ports AllocatedPorts) (collide bool) {
	for _, port := range ports {
		used := idx.getUsedPortsFor(port.HostIP)
		if port.Value < 0 || port.Value >= maxValidPort {
			return true
		}
		if used.Check(uint(port.Value)) {
			collide = true
		} else {
			used.Set(uint(port.Value))
		}
	}

	return
}

// AddReservedPortRange marks the ports given as reserved on all network
// interfaces. The port format is comma delimited, with spans given as n1-n2
// (80,100-200,205)
func (idx *NetworkIndex) AddReservedPortRange(ports string) (collide bool) {
	// Convert the ports into a slice of ints
	resPorts, err := ParsePortRanges(ports)
	if err != nil {
		return
	}

	// Ensure we create a bitmap for each available network
	for _, n := range idx.AvailNetworks {
		idx.getUsedPortsFor(n.IP)
	}

	for _, used := range idx.UsedPorts {
		for _, port := range resPorts {
			// Guard against invalid port
			if port < 0 || port >= maxValidPort {
				return true
			}
			if used.Check(uint(port)) {
				collide = true
			} else {
				used.Set(uint(port))
			}
		}
	}

	return
}

// AddReservedPortsForIP
func (idx *NetworkIndex) AddReservedPortsForIP(ports string, ip string) (collide bool) {
	// Convert the ports into a slice of ints
	resPorts, err := ParsePortRanges(ports)
	if err != nil {
		return
	}

	used := idx.getUsedPortsFor(ip)
	for _, port := range resPorts {
		// Guard against invalid port
		if port < 0 || port >= maxValidPort {
			return true
		}
		if used.Check(uint(port)) {
			collide = true
		} else {
			used.Set(uint(port))
		}
	}

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

func (idx *NetworkIndex) AssignPorts(ask *NetworkResource) (AllocatedPorts, error) {
	var offer AllocatedPorts

	// index of host network name to slice of reserved ports, used during dynamic port assignment
	reservedIdx := map[string][]Port{}

	for _, port := range ask.ReservedPorts {
		reservedIdx[port.HostNetwork] = append(reservedIdx[port.HostNetwork], port)

		// allocPort is set in the inner for loop if a port mapping can be created
		// if allocPort is still nil after the loop, the port wasn't available for reservation
		var allocPort *AllocatedPortMapping
		var addrErr error
		for _, addr := range idx.AvailAddresses[port.HostNetwork] {
			used := idx.getUsedPortsFor(addr.Address)
			// Guard against invalid port
			if port.Value < 0 || port.Value >= maxValidPort {
				return nil, fmt.Errorf("invalid port %d (out of range)", port.Value)
			}

			// Check if in use
			if used != nil && used.Check(uint(port.Value)) {
				return nil, fmt.Errorf("reserved port collision %s=%d", port.Label, port.Value)
			}

			allocPort = &AllocatedPortMapping{
				Label:  port.Label,
				Value:  port.Value,
				To:     port.To,
				HostIP: addr.Address,
			}
			break
		}

		if allocPort == nil {
			if addrErr != nil {
				return nil, addrErr
			}

			return nil, fmt.Errorf("no addresses available for %q network", port.HostNetwork)
		}

		offer = append(offer, *allocPort)
	}

	for _, port := range ask.DynamicPorts {
		var allocPort *AllocatedPortMapping
		var addrErr error
		for _, addr := range idx.AvailAddresses[port.HostNetwork] {
			used := idx.getUsedPortsFor(addr.Address)
			// Try to stochastically pick the dynamic ports as it is faster and
			// lower memory usage.
			var dynPorts []int
			// TODO: its more efficient to find multiple dynamic ports at once
			dynPorts, addrErr = getDynamicPortsStochastic(used, reservedIdx[port.HostNetwork], 1)
			if addrErr != nil {
				// Fall back to the precise method if the random sampling failed.
				dynPorts, addrErr = getDynamicPortsPrecise(used, reservedIdx[port.HostNetwork], 1)
				if addrErr != nil {
					continue
				}
			}

			allocPort = &AllocatedPortMapping{
				Label:  port.Label,
				Value:  dynPorts[0],
				To:     port.To,
				HostIP: addr.Address,
			}
			if allocPort.To == -1 {
				allocPort.To = allocPort.Value
			}
			break
		}

		if allocPort == nil {
			if addrErr != nil {
				return nil, addrErr
			}

			return nil, fmt.Errorf("no addresses available for %q network", port.HostNetwork)
		}
		offer = append(offer, *allocPort)
	}

	return offer, nil
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
				err = fmt.Errorf("reserved port collision %s=%d", port.Label, port.Value)
				return
			}
		}

		// Create the offer
		offer := &NetworkResource{
			Mode:          ask.Mode,
			Device:        n.Device,
			IP:            ipStr,
			MBits:         ask.MBits,
			DNS:           ask.DNS,
			ReservedPorts: ask.ReservedPorts,
			DynamicPorts:  ask.DynamicPorts,
		}

		// Try to stochastically pick the dynamic ports as it is faster and
		// lower memory usage.
		var dynPorts []int
		var dynErr error
		dynPorts, dynErr = getDynamicPortsStochastic(used, ask.ReservedPorts, len(ask.DynamicPorts))
		if dynErr == nil {
			goto BUILD_OFFER
		}

		// Fall back to the precise method if the random sampling failed.
		dynPorts, dynErr = getDynamicPortsPrecise(used, ask.ReservedPorts, len(ask.DynamicPorts))
		if dynErr != nil {
			err = dynErr
			return
		}

	BUILD_OFFER:
		for i, port := range dynPorts {
			offer.DynamicPorts[i].Value = port

			// This syntax allows you to set the mapped to port to the same port
			// allocated by the scheduler on the host.
			if offer.DynamicPorts[i].To == -1 {
				offer.DynamicPorts[i].To = port
			}
		}

		// Stop, we have an offer!
		out = offer
		err = nil
		return true
	})
	return
}

// getDynamicPortsPrecise takes the nodes used port bitmap which may be nil if
// no ports have been allocated yet, the network ask and returns a set of unused
// ports to fulfil the ask's DynamicPorts or an error if it failed. An error
// means the ask can not be satisfied as the method does a precise search.
func getDynamicPortsPrecise(nodeUsed Bitmap, reserved []Port, numDyn int) ([]int, error) {
	// Create a copy of the used ports and apply the new reserves
	var usedSet Bitmap
	var err error
	if nodeUsed != nil {
		usedSet, err = nodeUsed.Copy()
		if err != nil {
			return nil, err
		}
	} else {
		usedSet, err = NewBitmap(maxValidPort)
		if err != nil {
			return nil, err
		}
	}

	for _, port := range reserved {
		usedSet.Set(uint(port.Value))
	}

	// Get the indexes of the unset
	dynamicPortRange := GetDynamicPortRange()
	availablePorts := usedSet.IndexesInRange(false, uint(dynamicPortRange.Min), uint(dynamicPortRange.Max))

	// Randomize the amount we need
	if len(availablePorts) < numDyn {
		return nil, fmt.Errorf("dynamic port selection failed")
	}

	numAvailable := len(availablePorts)
	for i := 0; i < numDyn; i++ {
		j := rand.Intn(numAvailable)
		availablePorts[i], availablePorts[j] = availablePorts[j], availablePorts[i]
	}

	return availablePorts[:numDyn], nil
}

// getDynamicPortsStochastic takes the nodes used port bitmap which may be nil if
// no ports have been allocated yet, the network ask and returns a set of unused
// ports to fulfil the ask's DynamicPorts or an error if it failed. An error
// does not mean the ask can not be satisfied as the method has a fixed amount
// of random probes and if these fail, the search is aborted.
func getDynamicPortsStochastic(nodeUsed Bitmap, reservedPorts []Port, count int) ([]int, error) {
	var reserved, dynamic []int
	for _, port := range reservedPorts {
		reserved = append(reserved, port.Value)
	}

	for i := 0; i < count; i++ {
		attempts := 0
	PICK:
		attempts++
		if attempts > maxRandPortAttempts {
			return nil, fmt.Errorf("stochastic dynamic port selection failed")
		}

		dynamicPortRange := GetDynamicPortRange()
		randPort := dynamicPortRange.Min + rand.Intn(dynamicPortRange.Max-dynamicPortRange.Min)
		if nodeUsed != nil && nodeUsed.Check(uint(randPort)) {
			goto PICK
		}

		for _, ports := range [][]int{reserved, dynamic} {
			if isPortReserved(ports, randPort) {
				goto PICK
			}
		}
		dynamic = append(dynamic, randPort)
	}

	return dynamic, nil
}

// IntContains scans an integer slice for a value
func isPortReserved(haystack []int, needle int) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// COMPAT(1.0) remove when NetworkResource is no longer used for materialized client view of ports
func AllocatedPortsToNetworkResouce(ask *NetworkResource, ports AllocatedPorts, node *NodeResources) *NetworkResource {
	out := ask.Copy()

	for i, port := range ask.DynamicPorts {
		if p, ok := ports.Get(port.Label); ok {
			out.DynamicPorts[i].Value = p.Value
			out.DynamicPorts[i].To = p.To
		}
	}
	if len(node.NodeNetworks) > 0 {
		for _, nw := range node.NodeNetworks {
			if nw.Mode == "host" {
				out.IP = nw.Addresses[0].Address
				break
			}
		}
	} else {
		for _, nw := range node.Networks {
			if nw.Mode == "host" {
				out.IP = nw.IP
			}
		}
	}
	return out
}

type ClientHostNetworkConfig struct {
	Name          string `hcl:",key"`
	CIDR          string `hcl:"cidr"`
	Interface     string `hcl:"interface"`
	ReservedPorts string `hcl:"reserved_ports"`
}
