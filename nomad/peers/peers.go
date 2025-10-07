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

	// Deprecated: Functionally unused but needs to always be set by 1 for
	// compatibility with v1.2.x and earlier.
	MajorVersion int
}

func (p *Parts) String() string {
	return fmt.Sprintf("%s (Addr: %s) (DC: %s)", p.Name, p.Addr, p.Datacenter)
}

func (p *Parts) Copy() *Parts {
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
		MajorVersion: deprecatedAPIMajorVersion,
	}
	return true, parts
}

// PartCache is a threadsafe cache of known Nomad server peers parsed from Serf
// members. It avoids the need to re-parse Serf members each time the peers
// need to be inspected.
type PartCache struct {

	// peers is a map of region names to the list of known server peers in that
	// region. All access must be protected by peersLock.
	peers     map[string][]*Parts
	peersLock sync.RWMutex
}

// NewPartsCache returns a new instance of a PartsCache ready for use.
func NewPartsCache() *PartCache {
	return &PartCache{
		peers: make(map[string][]*Parts),
	}
}

// PeerSet adds or updates the given parts in the cache. This should be called
// when a new peer is detected or an existing peer changes is status.
func (p *PartCache) PeerSet(parts *Parts) {
	p.peersLock.Lock()
	defer p.peersLock.Unlock()

	existing, ok := p.peers[parts.Region]
	if !ok {
		p.peers[parts.Region] = []*Parts{parts}
		return
	}

	// Replace if already present
	for i, ep := range existing {
		if ep.Name == parts.Name {
			existing[i] = parts
			return
		}
	}

	// If we reached this point then it's a new member, so append it to the
	// exiting array.
	p.peers[parts.Region] = append(existing, parts)
}

// PeerDelete removes the given members from the cache. This should be called
// when a peer is reaped from the Serf cluster.
func (p *PartCache) PeerDelete(event serf.MemberEvent) {
	p.peersLock.Lock()
	defer p.peersLock.Unlock()

	for _, m := range event.Members {
		if ok, parts := IsNomadServer(m); ok {
			existing := p.peers[parts.Region]
			n := len(existing)
			for i := 0; i < n; i++ {
				if existing[i].Name == parts.Name {
					existing[i], existing[n-1] = existing[n-1], nil
					existing = existing[:n-1]
					n--
					break
				}
			}

			// If all peers in the region are gone, remove the region entry
			// entirely. Otherwise, update the list.
			if n < 1 {
				delete(p.peers, parts.Region)
			} else {
				p.peers[parts.Region] = existing
			}
		}
	}
}

// ServersMeetMinimumVersion can be used to check whether the known server peers
// meet the given minimum version. The region can be set to a specific region
// or to AllRegions to check all known regions. If checkFailedServers is true
// then servers in the Failed state will also be checked, otherwise only servers
// in the Alive state are considered.
func (p *PartCache) ServersMeetMinimumVersion(
	region string,
	minVersion *version.Version,
	checkFailedServers bool,
) bool {

	// Copy the peers under lock to avoid holding the lock while doing the check
	// which could be slow if there are many peers.
	p.peersLock.RLock()
	peers := p.peers
	p.peersLock.RUnlock()

	// If the caller wants to check all regions, do so, otherwise test against
	// the specific region.
	switch region {
	case AllRegions:
		for _, peerList := range peers {
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
		peerList, ok := peers[region]
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
