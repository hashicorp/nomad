package drivers

import (
	"fmt"
	"hash/crc32"
	"maps"
	"net"
	"slices"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/plugin-interface/lib/idset"
)

type AllocatedDevices []*AllocatedDeviceResource

// Index finds the matching index using the passed device. If not found, -1 is
// returned.
func (a AllocatedDevices) Index(d *AllocatedDeviceResource) int {
	if d == nil {
		return -1
	}

	for i, o := range a {
		if o.ID().Equal(d.ID()) {
			return i
		}
	}

	return -1
}

// AllocatedTaskResources are the set of resources allocated to a task.
type AllocatedTaskResources struct {
	Cpu      AllocatedCpuResources
	Memory   AllocatedMemoryResources
	Networks Networks
	Devices  []*AllocatedDeviceResource
}

func (a *AllocatedTaskResources) Copy() *AllocatedTaskResources {
	if a == nil {
		return nil
	}
	newA := new(AllocatedTaskResources)
	*newA = *a

	// Copy the networks
	newA.Networks = a.Networks.Copy()

	// Copy the devices
	if newA.Devices != nil {
		n := len(a.Devices)
		newA.Devices = make([]*AllocatedDeviceResource, n)
		for i := range n {
			newA.Devices[i] = a.Devices[i].Copy()
		}
	}

	return newA
}

// NetIndex finds the matching net index using device name
func (a *AllocatedTaskResources) NetIndex(n *NetworkResource) int {
	return a.Networks.NetIndex(n)
}

func (a *AllocatedTaskResources) Add(delta *AllocatedTaskResources) {
	if delta == nil {
		return
	}

	a.Cpu.Add(&delta.Cpu)
	a.Memory.Add(&delta.Memory)

	for _, n := range delta.Networks {
		// Find the matching interface by IP or CIDR
		idx := a.NetIndex(n)
		if idx == -1 {
			a.Networks = append(a.Networks, n.Copy())
		} else {
			a.Networks[idx].Add(n)
		}
	}

	for _, d := range delta.Devices {
		// Find the matching device
		idx := AllocatedDevices(a.Devices).Index(d)
		if idx == -1 {
			a.Devices = append(a.Devices, d.Copy())
		} else {
			a.Devices[idx].Add(d)
		}
	}
}

func (a *AllocatedTaskResources) Max(other *AllocatedTaskResources) {
	if other == nil {
		return
	}

	a.Cpu.Max(&other.Cpu)
	a.Memory.Max(&other.Memory)

	for _, n := range other.Networks {
		// Find the matching interface by IP or CIDR
		idx := a.NetIndex(n)
		if idx == -1 {
			a.Networks = append(a.Networks, n.Copy())
		} else {
			a.Networks[idx].Add(n)
		}
	}

	for _, d := range other.Devices {
		// Find the matching device
		idx := AllocatedDevices(a.Devices).Index(d)
		if idx == -1 {
			a.Devices = append(a.Devices, d.Copy())
		} else {
			a.Devices[idx].Add(d)
		}
	}
}

// Comparable turns AllocatedTaskResources into ComparableResources
// as a helper step in preemption
func (a *AllocatedTaskResources) Comparable() *ComparableResources {
	ret := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares:     a.Cpu.CpuShares,
				ReservedCores: a.Cpu.ReservedCores,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB:    a.Memory.MemoryMB,
				MemoryMaxMB: a.Memory.MemoryMaxMB,
			},
		},
	}
	ret.Flattened.Networks = append(ret.Flattened.Networks, a.Networks...)
	return ret
}

// Subtract only subtracts CPU and Memory resources. Network utilization
// is managed separately in NetworkIndex
func (a *AllocatedTaskResources) Subtract(delta *AllocatedTaskResources) {
	if delta == nil {
		return
	}

	a.Cpu.Subtract(&delta.Cpu)
	a.Memory.Subtract(&delta.Memory)
}

// AllocatedMemoryResources captures the allocated memory resources.
type AllocatedMemoryResources struct {
	MemoryMB    int64
	MemoryMaxMB int64
}

func (a *AllocatedMemoryResources) Add(delta *AllocatedMemoryResources) {
	if delta == nil {
		return
	}

	a.MemoryMB += delta.MemoryMB
	if delta.MemoryMaxMB != 0 {
		a.MemoryMaxMB += delta.MemoryMaxMB
	} else {
		a.MemoryMaxMB += delta.MemoryMB
	}
}

func (a *AllocatedMemoryResources) Subtract(delta *AllocatedMemoryResources) {
	if delta == nil {
		return
	}

	a.MemoryMB -= delta.MemoryMB
	if delta.MemoryMaxMB != 0 {
		a.MemoryMaxMB -= delta.MemoryMaxMB
	} else {
		a.MemoryMaxMB -= delta.MemoryMB
	}
}

func (a *AllocatedMemoryResources) Max(other *AllocatedMemoryResources) {
	if other == nil {
		return
	}

	if other.MemoryMB > a.MemoryMB {
		a.MemoryMB = other.MemoryMB
	}
	if other.MemoryMaxMB > a.MemoryMaxMB {
		a.MemoryMaxMB = other.MemoryMaxMB
	}
}

// AllocatedCpuResources captures the allocated CPU resources.
type AllocatedCpuResources struct {
	CpuShares     int64
	ReservedCores []uint16
}

func (a *AllocatedCpuResources) Add(delta *AllocatedCpuResources) {
	if delta == nil {
		return
	}

	// add cpu bandwidth
	a.CpuShares += delta.CpuShares

	// add cpu cores
	cores := idset.From[uint16](a.ReservedCores)
	deltaCores := idset.From[uint16](delta.ReservedCores)
	cores.InsertSet(deltaCores)
	a.ReservedCores = cores.Slice()
}

func (a *AllocatedCpuResources) Subtract(delta *AllocatedCpuResources) {
	if delta == nil {
		return
	}

	// remove cpu bandwidth
	a.CpuShares -= delta.CpuShares

	// remove cpu cores
	cores := idset.From[uint16](a.ReservedCores)
	deltaCores := idset.From[uint16](delta.ReservedCores)
	cores.RemoveSet(deltaCores)
	a.ReservedCores = cores.Slice()
}

func (a *AllocatedCpuResources) Max(other *AllocatedCpuResources) {
	if other == nil {
		return
	}

	if other.CpuShares > a.CpuShares {
		a.CpuShares = other.CpuShares
	}

	if len(other.ReservedCores) > len(a.ReservedCores) {
		a.ReservedCores = other.ReservedCores
	}
}

type AllocatedPortMapping struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	Label           string
	Value           int
	To              int
	HostIP          string
	IgnoreCollision bool
}

func (m *AllocatedPortMapping) Copy() *AllocatedPortMapping {
	return &AllocatedPortMapping{
		Label:           m.Label,
		Value:           m.Value,
		To:              m.To,
		HostIP:          m.HostIP,
		IgnoreCollision: m.IgnoreCollision,
	}
}

func (m *AllocatedPortMapping) Equal(o *AllocatedPortMapping) bool {
	if m == nil || o == nil {
		return m == o
	}
	switch {
	case m.Label != o.Label:
		return false
	case m.Value != o.Value:
		return false
	case m.To != o.To:
		return false
	case m.HostIP != o.HostIP:
		return false
	case m.IgnoreCollision != o.IgnoreCollision:
		return false
	}
	return true
}

type AllocatedPorts []AllocatedPortMapping

func (p AllocatedPorts) Equal(o AllocatedPorts) bool {
	return slices.EqualFunc(p, o, func(a, b AllocatedPortMapping) bool {
		return a.Equal(&b)
	})
}

func (p AllocatedPorts) Get(label string) (AllocatedPortMapping, bool) {
	for _, port := range p {
		if port.Label == label {
			return port, true
		}
	}

	return AllocatedPortMapping{}, false
}

type Port struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// Label is the key for HCL port blocks: port "foo" {}
	Label string

	// Value is the static or dynamic port value. For dynamic ports this
	// will be 0 in the jobspec and set by the scheduler.
	Value int

	// To is the port inside a network namespace where this port is
	// forwarded. -1 is an internal sentinel value used by Consul Connect
	// to mean "same as the host port."
	To int

	// HostNetwork is the name of the network this port should be assigned
	// to. Jobs with a HostNetwork set can only be placed on nodes with
	// that host network available.
	HostNetwork string

	// IgnoreCollision ignores port collisions, so the port can be used more
	// than one time on a single network, for tasks that support SO_REUSEPORT
	// Should be used only with static ports.
	IgnoreCollision bool
}

// AllocatedDeviceResource captures a set of allocated devices.
type AllocatedDeviceResource struct {
	// Vendor, Type, and Name are used to select the plugin to request the
	// device IDs from.
	Vendor string
	Type   string
	Name   string

	// DeviceIDs is the set of allocated devices
	DeviceIDs []string
}

func (a *AllocatedDeviceResource) ID() *DeviceIdTuple {
	if a == nil {
		return nil
	}

	return &DeviceIdTuple{
		Vendor: a.Vendor,
		Type:   a.Type,
		Name:   a.Name,
	}
}

func (a *AllocatedDeviceResource) Add(delta *AllocatedDeviceResource) {
	if delta == nil {
		return
	}

	a.DeviceIDs = append(a.DeviceIDs, delta.DeviceIDs...)
}

func (a *AllocatedDeviceResource) Copy() *AllocatedDeviceResource {
	if a == nil {
		return a
	}

	na := *a

	// Copy the devices
	na.DeviceIDs = make([]string, len(a.DeviceIDs))
	copy(na.DeviceIDs, a.DeviceIDs)
	return &na
}

// DeviceIdTuple is the tuple that identifies a device
type DeviceIdTuple struct {
	Vendor string
	Type   string
	Name   string
}

func (id *DeviceIdTuple) String() string {
	if id == nil {
		return ""
	}

	return fmt.Sprintf("%s/%s/%s", id.Vendor, id.Type, id.Name)
}

// Matches returns if this Device ID is a superset of the passed ID.
func (id *DeviceIdTuple) Matches(other *DeviceIdTuple) bool {
	if other == nil {
		return false
	}

	if other.Name != "" && other.Name != id.Name {
		return false
	}

	if other.Vendor != "" && other.Vendor != id.Vendor {
		return false
	}

	if other.Type != "" && other.Type != id.Type {
		return false
	}

	return true
}

// Equal returns if this Device ID is the same as the passed ID.
func (id *DeviceIdTuple) Equal(o *DeviceIdTuple) bool {
	if id == nil && o == nil {
		return true
	} else if id == nil || o == nil {
		return false
	}

	return o.Vendor == id.Vendor && o.Type == id.Type && o.Name == id.Name
}

type CNIConfig struct {
	Args map[string]string
}

func (d *CNIConfig) Copy() *CNIConfig {
	if d == nil {
		return nil
	}
	newMap := make(map[string]string)
	maps.Copy(newMap, d.Args)
	return &CNIConfig{
		Args: newMap,
	}
}

func (d *CNIConfig) Equal(o *CNIConfig) bool {
	if d == nil || o == nil {
		return d == o
	}
	return maps.Equal(d.Args, o.Args)
}

// NetworkResource is used to represent available network
// resources
type NetworkResource struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	Mode          string     // Mode of the network
	Device        string     // Name of the device
	CIDR          string     // CIDR block of addresses
	IP            string     // Host IP address
	Hostname      string     `json:",omitempty"` // Hostname of the network namespace
	MBits         int        // Throughput
	DNS           *DNSConfig // DNS Configuration
	ReservedPorts []Port     // Host Reserved ports
	DynamicPorts  []Port     // Host Dynamically assigned ports
	CNI           *CNIConfig // CNIConfig Configuration
}

func (n *NetworkResource) Hash() uint32 {
	var data []byte
	data = fmt.Appendf(data, "%s%s%s%s%s%d", n.Mode, n.Device, n.CIDR, n.IP, n.Hostname, n.MBits)

	for i, port := range n.ReservedPorts {
		data = fmt.Appendf(data, "r%d%s%d%d", i, port.Label, port.Value, port.To)
	}

	for i, port := range n.DynamicPorts {
		data = fmt.Appendf(data, "d%d%s%d%d", i, port.Label, port.Value, port.To)
	}

	return crc32.ChecksumIEEE(data)
}

func (n *NetworkResource) Equal(other *NetworkResource) bool {
	return n.Hash() == other.Hash()
}

func (n *NetworkResource) Canonicalize() {
	// Ensure that an empty and nil slices are treated the same to avoid scheduling
	// problems since we use reflect DeepEquals.
	if len(n.ReservedPorts) == 0 {
		n.ReservedPorts = nil
	}
	if len(n.DynamicPorts) == 0 {
		n.DynamicPorts = nil
	}

	for i, p := range n.DynamicPorts {
		if p.HostNetwork == "" {
			n.DynamicPorts[i].HostNetwork = "default"
		}
	}
	for i, p := range n.ReservedPorts {
		if p.HostNetwork == "" {
			n.ReservedPorts[i].HostNetwork = "default"
		}
	}
}

// Copy returns a deep copy of the network resource
func (n *NetworkResource) Copy() *NetworkResource {
	if n == nil {
		return nil
	}
	newR := new(NetworkResource)
	*newR = *n
	newR.DNS = n.DNS.Copy()
	if n.ReservedPorts != nil {
		newR.ReservedPorts = make([]Port, len(n.ReservedPorts))
		copy(newR.ReservedPorts, n.ReservedPorts)
	}
	if n.DynamicPorts != nil {
		newR.DynamicPorts = make([]Port, len(n.DynamicPorts))
		copy(newR.DynamicPorts, n.DynamicPorts)
	}
	return newR
}

// Add adds the resources of the delta to this, potentially
// returning an error if not possible.
func (n *NetworkResource) Add(delta *NetworkResource) {
	if len(delta.ReservedPorts) > 0 {
		n.ReservedPorts = append(n.ReservedPorts, delta.ReservedPorts...)
	}
	n.MBits += delta.MBits
	n.DynamicPorts = append(n.DynamicPorts, delta.DynamicPorts...)
}

func (n *NetworkResource) GoString() string {
	return fmt.Sprintf("*%#v", *n)
}

// PortLabels returns a map of port labels to their assigned host ports.
func (n *NetworkResource) PortLabels() map[string]int {
	num := len(n.ReservedPorts) + len(n.DynamicPorts)
	labelValues := make(map[string]int, num)
	for _, port := range n.ReservedPorts {
		labelValues[port.Label] = port.Value
	}
	for _, port := range n.DynamicPorts {
		labelValues[port.Label] = port.Value
	}
	return labelValues
}

func (n *NetworkResource) IsIPv6() bool {
	ip := net.ParseIP(n.IP)
	return ip != nil && ip.To4() == nil
}

// Networks defined for a task on the Resources struct.
type Networks []*NetworkResource

func (ns Networks) Copy() Networks {
	if len(ns) == 0 {
		return nil
	}

	out := make([]*NetworkResource, len(ns))
	for i := range ns {
		out[i] = ns[i].Copy()
	}
	return out
}

// Port assignment and IP for the given label or empty values.
func (ns Networks) Port(label string) AllocatedPortMapping {
	for _, n := range ns {
		for _, p := range n.ReservedPorts {
			if p.Label == label {
				return AllocatedPortMapping{
					Label:           label,
					Value:           p.Value,
					To:              p.To,
					HostIP:          n.IP,
					IgnoreCollision: p.IgnoreCollision,
				}
			}
		}
		for _, p := range n.DynamicPorts {
			if p.Label == label {
				return AllocatedPortMapping{
					Label:  label,
					Value:  p.Value,
					To:     p.To,
					HostIP: n.IP,
				}
			}
		}
	}
	return AllocatedPortMapping{}
}

func (ns Networks) NetIndex(n *NetworkResource) int {
	for idx, net := range ns {
		if net.Device == n.Device {
			return idx
		}
	}
	return -1
}

// Modes returns the set of network modes used by our NetworkResource blocks.
func (ns Networks) Modes() *set.Set[string] {
	return set.FromFunc(ns, func(nr *NetworkResource) string {
		return nr.Mode
	})
}

// ComparableResources is the set of resources allocated to a task group but
// not keyed by Task, making it easier to compare.
type ComparableResources struct {
	Flattened AllocatedTaskResources
	Shared    AllocatedSharedResources
}

func (c *ComparableResources) Add(delta *ComparableResources) {
	if delta == nil {
		return
	}

	c.Flattened.Add(&delta.Flattened)
	c.Shared.Add(&delta.Shared)
}

func (c *ComparableResources) Subtract(delta *ComparableResources) {
	if delta == nil {
		return
	}

	c.Flattened.Subtract(&delta.Flattened)
	c.Shared.Subtract(&delta.Shared)
}

func (c *ComparableResources) Copy() *ComparableResources {
	if c == nil {
		return nil
	}
	newR := new(ComparableResources)
	*newR = *c
	return newR
}

// Superset checks if one set of resources is a superset of another. This
// ignores network resources, and the NetworkIndex should be used for that.
func (c *ComparableResources) Superset(other *ComparableResources) (bool, string) {
	if c.Flattened.Cpu.CpuShares < other.Flattened.Cpu.CpuShares {
		return false, "cpu"
	}

	cores := idset.From[uint16](c.Flattened.Cpu.ReservedCores)
	otherCores := idset.From[uint16](other.Flattened.Cpu.ReservedCores)
	if len(c.Flattened.Cpu.ReservedCores) > 0 && !cores.Superset(otherCores) {
		return false, "cores"
	}

	if c.Flattened.Memory.MemoryMB < other.Flattened.Memory.MemoryMB {
		return false, "memory"
	}

	if c.Shared.DiskMB < other.Shared.DiskMB {
		return false, "disk"
	}
	return true, ""
}

// NetIndex finds the matching net index using device name
func (c *ComparableResources) NetIndex(n *NetworkResource) int {
	return c.Flattened.Networks.NetIndex(n)
}

// AllocatedSharedResources are the set of resources allocated to a task group.
type AllocatedSharedResources struct {
	Networks Networks
	DiskMB   int64
	Ports    AllocatedPorts
}

func (a AllocatedSharedResources) Copy() AllocatedSharedResources {
	return AllocatedSharedResources{
		Networks: a.Networks.Copy(),
		DiskMB:   a.DiskMB,
		Ports:    a.Ports,
	}
}

func (a *AllocatedSharedResources) Add(delta *AllocatedSharedResources) {
	if delta == nil {
		return
	}
	a.Networks = append(a.Networks, delta.Networks...)
	a.DiskMB += delta.DiskMB

}

func (a *AllocatedSharedResources) Subtract(delta *AllocatedSharedResources) {
	if delta == nil {
		return
	}

	diff := map[*NetworkResource]bool{}
	for _, n := range delta.Networks {
		diff[n] = true
	}
	var nets Networks
	for _, n := range a.Networks {
		if _, ok := diff[n]; !ok {
			nets = append(nets, n)
		}
	}
	a.Networks = nets
	a.DiskMB -= delta.DiskMB
}

func (a *AllocatedSharedResources) Canonicalize() {
	if len(a.Networks) > 0 {
		if len(a.Networks[0].DynamicPorts)+len(a.Networks[0].ReservedPorts) > 0 && len(a.Ports) == 0 {
			for _, ports := range [][]Port{a.Networks[0].DynamicPorts, a.Networks[0].ReservedPorts} {
				for _, p := range ports {
					a.Ports = append(a.Ports, AllocatedPortMapping{
						Label:  p.Label,
						Value:  p.Value,
						To:     p.To,
						HostIP: a.Networks[0].IP,
					})
				}
			}
		}
	}
}
