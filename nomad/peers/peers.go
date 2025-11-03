// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peers

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

const (
	// Deprecated: Through Nomad v1.2 these values were configurable but
	// functionally unused. We still need to advertise them in Serf for
	// compatibility with v1.2 and earlier.
	deprecatedAPIMajorVersion = 1

	// AllRegions is the special region name that can be used within the
	// ServersMeetMinimumVersion to perform the check across all federatd
	// regions.
	AllRegions = ""
)

// Parts is the parsed Serf member information for a Nomad server.
type Parts struct {
	Name        string
	ID          string
	Region      string
	Datacenter  string
	Port        int
	Bootstrap   bool
	Expect      int
	Build       version.Version
	RaftVersion int
	Addr        net.Addr
	RPCAddr     net.Addr
	Status      serf.MemberStatus
	NonVoter    bool

	// Tags are the full Serf tags for the member. This duplicates some of the
	// data which is also stored in other fields for convenience, so adds a
	// little size overhead. It is included as Autopilot uses them.
	Tags map[string]string

	// Deprecated: Functionally unused but needs to always be set by 1 for
	// compatibility with v1.2.x and earlier.
	MajorVersion int
}

func (p *Parts) String() string {
	return fmt.Sprintf("%s (Addr: %s) (DC: %s)", p.Name, p.Addr, p.Datacenter)
}

func (p *Parts) Copy() *Parts {
	if p == nil {
		return nil
	}
	ns := new(Parts)
	*ns = *p
	return ns
}

// IsNomadServer returns whether the given Serf member is a Nomad server and its
// parts if so.
func IsNomadServer(m serf.Member) (bool, *Parts) {
	if m.Tags["role"] != "nomad" {
		return false, nil
	}

	id := "unknown"
	if v, ok := m.Tags["id"]; ok {
		id = v
	}
	region := m.Tags["region"]
	datacenter := m.Tags["dc"]
	_, bootstrap := m.Tags["bootstrap"]

	expect := 0
	expectStr, ok := m.Tags["expect"]
	var err error
	if ok {
		expect, err = strconv.Atoi(expectStr)
		if err != nil {
			return false, nil
		}
	}

	// If the server is missing the rpc_addr tag, default to the serf advertise
	// addr.
	rpcIP := net.ParseIP(m.Tags["rpc_addr"])
	if rpcIP == nil {
		rpcIP = m.Addr
	}

	portStr := m.Tags["port"]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false, nil
	}

	buildVersion, err := version.NewVersion(m.Tags["build"])
	if err != nil {
		return false, nil
	}

	raftVsn := 0
	raftVsnString, ok := m.Tags["raft_vsn"]
	if ok {
		raftVsn, err = strconv.Atoi(raftVsnString)
		if err != nil {
			return false, nil
		}
	}

	// Check if the server is a non voter
	_, nonVoter := m.Tags["nonvoter"]

	addr := &net.TCPAddr{IP: m.Addr, Port: port}
	rpcAddr := &net.TCPAddr{IP: rpcIP, Port: port}
	parts := &Parts{
		Name:         m.Name,
		ID:           id,
		Region:       region,
		Datacenter:   datacenter,
		Port:         port,
		Bootstrap:    bootstrap,
		Expect:       expect,
		Addr:         addr,
		RPCAddr:      rpcAddr,
		Build:        *buildVersion,
		RaftVersion:  raftVsn,
		Status:       m.Status,
		NonVoter:     nonVoter,
		Tags:         m.Tags,
		MajorVersion: deprecatedAPIMajorVersion,
	}
	return true, parts
}

// PartCache is a threadsafe cache of known Nomad server peers parsed from Serf
// members. It avoids the need to re-parse Serf members each time the peers need
// to be inspected and can be used for RPC routing and discovery and server
// version checking.
//
// Returned peer objects are copies of the cached objects. This ensures that the
// peer object is not mutated while being used by the caller.
type PeerCache struct {

	// allPeers is a map of region names to the list of known server peers in
	// that region. Peers stored here can be in failed states and is used for
	// server version checking only.
	allPeers map[string][]*Parts

	// alivePeers is a map of region names to the list of known server peers in
	// that region. Peers stored here are only those in the Alive state and is
	// used for RPC routing and discovery.
	alivePeers map[string][]*Parts

	// localPeers is a map of the known server peers in the local region keyed
	// by their Raft address. Peers stored here are only those in the Alive
	// state and is used for intra-region RPC routing and discovery.
	localPeers map[raft.ServerAddress]*Parts

	// peersLock protects access to the maps above. We use a single lock so that
	// updates are correctly applied to all maps together.
	peersLock sync.RWMutex
}

// NewPeerCache returns a new instance of a PeerCache ready for use.
func NewPeerCache() *PeerCache {
	return &PeerCache{
		allPeers:   make(map[string][]*Parts),
		alivePeers: make(map[string][]*Parts),
		localPeers: make(map[raft.ServerAddress]*Parts),
	}
}

func (p *PeerCache) LocalPeer(addr raft.ServerAddress) *Parts {
	p.peersLock.RLock()
	defer p.peersLock.RUnlock()
	return p.localPeers[addr].Copy()
}

// LocalPeers returns a list of known alive peers in the local region.
func (p *PeerCache) LocalPeers() []*Parts {
	p.peersLock.RLock()
	defer p.peersLock.RUnlock()

	peers := make([]*Parts, 0, len(p.localPeers))

	for _, peer := range p.localPeers {
		peers = append(peers, peer.Copy())
	}

	return peers
}

func (p *PeerCache) LocalPeersServerInfo() []*structs.NodeServerInfo {
	p.peersLock.RLock()
	defer p.peersLock.RUnlock()

	peers := make([]*structs.NodeServerInfo, 0, len(p.localPeers))

	for _, peer := range p.localPeers {
		peers = append(peers, &structs.NodeServerInfo{
			RPCAdvertiseAddr: peer.RPCAddr.String(),
			Datacenter:       peer.Datacenter,
		})
	}

	return peers
}

// RegionNum returns the number of known regions with at least one alive peer
// and are therfore suitable for RPC routing.
func (p *PeerCache) RegionNum() int {
	p.peersLock.RLock()
	defer p.peersLock.RUnlock()
	return len(p.alivePeers)
}

// RegionNames returns the names of all known regions with at least one alive
// peer and are therefore suitable for RPC routing.
func (p *PeerCache) RegionNames() []string {
	p.peersLock.RLock()
	defer p.peersLock.RUnlock()

	names := make([]string, 0, len(p.alivePeers))
	for name := range p.alivePeers {
		names = append(names, name)
	}

	return names
}

// RegionPeers returns the list of known alive peers in the given region. If the
// region is not known or has no alive peers, an empty list is returned.
func (p *PeerCache) RegionPeers(region string) []*Parts {
	p.peersLock.RLock()
	defer p.peersLock.RUnlock()

	numPeers := len(p.alivePeers[region])
	if numPeers == 0 {
		return nil
	}

	peers := make([]*Parts, 0, numPeers)

	for _, peer := range p.alivePeers[region] {
		peers = append(peers, peer.Copy())
	}
	return peers
}

// PeerSet adds or updates the given parts in the cache. This should be called
// when a new peer is detected or an existing peer changes is status.
func (p *PeerCache) PeerSet(parts *Parts, localRegion string) {
	p.peersLock.Lock()
	defer p.peersLock.Unlock()

	// Mirror the update in the all peers mapping which tracks all known peers
	// regardless of status.
	p.peerSetLocked(p.allPeers, parts)

	// Now update the alive peers and local peers mappings based on the status.
	switch parts.Status {
	case serf.StatusAlive:
		p.peerSetLocked(p.alivePeers, parts)
		p.peerSetLocalLocked(parts, localRegion)
	default:
		p.peerDeleteLocked(p.alivePeers, parts)
		p.peerDeleteLocalLocked(parts, localRegion)
	}
}

// peerSetLocalLocked adds or updates the given parts in the local peers map if
// it is in the local region. The caller must hold the peersLock.
func (p *PeerCache) peerSetLocalLocked(parts *Parts, localRegion string) {
	if parts.Region == localRegion {
		p.localPeers[raft.ServerAddress(parts.Addr.String())] = parts
	}
}

func (p *PeerCache) peerSetLocked(peers map[string][]*Parts, parts *Parts) {

	// Track if we found the peer already in the list.
	var found bool

	existing := peers[parts.Region]

	// Replace if already present
	for i, ep := range existing {
		if ep.Name == parts.Name {
			existing[i] = parts
			found = true
			break
		}
	}

	// Add ot the list if not known
	if !found {
		peers[parts.Region] = append(existing, parts)
	}
}

// PeerDelete removes the given members from the cache. This should be called
// when a peer is reaped from the Serf cluster.
func (p *PeerCache) PeerDelete(event serf.MemberEvent) {
	p.peersLock.Lock()
	defer p.peersLock.Unlock()

	for _, m := range event.Members {
		if ok, parts := IsNomadServer(m); ok {
			p.peerDeleteLocked(p.allPeers, parts)
		}
	}
}

// peerDeleteLocalLocked removes the given parts from the local peers map if it
// is in the local region. The caller must hold the peersLock.
func (p *PeerCache) peerDeleteLocalLocked(parts *Parts, localRegion string) {
	if parts.Region == localRegion {
		delete(p.localPeers, raft.ServerAddress(parts.Addr.String()))
	}
}

func (p *PeerCache) peerDeleteLocked(peers map[string][]*Parts, parts *Parts) {

	existing := peers[parts.Region]

	existing = slices.DeleteFunc(
		existing,
		func(member *Parts) bool { return member.Name == parts.Name },
	)

	// If all peers in the region are gone, remove the region entry
	// entirely. Otherwise, update the list.
	if len(existing) < 1 {
		delete(peers, parts.Region)
	} else {
		peers[parts.Region] = existing
	}
}

// ServersMeetMinimumVersion can be used to check whether the known server peers
// meet the given minimum version. The region can be set to a specific region
// or to AllRegions to check all known regions. If checkFailedServers is true
// then servers in the Failed state will also be checked, otherwise only servers
// in the Alive state are considered.
func (p *PeerCache) ServersMeetMinimumVersion(
	region string,
	minVersion *version.Version,
	checkFailedServers bool,
) bool {

	// Acquire the read lock to access the peers map. It would be possible to
	// copy the map and slices to avoid holding the lock for the entire function
	// duration, but the time overhead of iterating the map and slices for the
	// copy would be close to the time taken to just hold the lock for the
	// duration of this function.
	p.peersLock.RLock()
	defer p.peersLock.RUnlock()

	// If the caller wants to check all regions, do so, otherwise test against
	// the specific region.
	switch region {
	case AllRegions:
		for _, peerList := range p.allPeers {
			if !regionServersMeetMinimumVersion(peerList, minVersion, checkFailedServers) {
				return false
			}
		}
		return true
	default:
		// At the time of writing, version checks are either done against the
		// local region or all regions only. It's not possible that the server
		// is querying its own region but that region is not known. However, in
		// the future we may change this, so guard against it here just in case.
		peerList, ok := p.allPeers[region]
		if !ok {
			return false
		}
		return regionServersMeetMinimumVersion(peerList, minVersion, checkFailedServers)
	}
}

func regionServersMeetMinimumVersion(
	peers []*Parts,
	minVersion *version.Version,
	checkFailedServers bool,
) bool {

	for _, parts := range peers {
		if parts.Status != serf.StatusAlive && !(checkFailedServers && parts.Status == serf.StatusFailed) {
			continue
		}
		versionsMatch := slices.Equal(minVersion.Segments(), parts.Build.Segments())
		if parts.Build.LessThan(minVersion) && !versionsMatch {
			return false
		}
	}
	return true
}
