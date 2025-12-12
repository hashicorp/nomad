// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package peers

import (
	"fmt"
	"net"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/serf/serf"
	"github.com/shoenig/test/must"
)

func TestParts_String(t *testing.T) {
	ci.Parallel(t)

	parts := &Parts{
		Name:       "foo",
		Datacenter: "dc",
		Addr:       &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
	}

	must.StrEqFold(t, "foo (Addr: 127.0.0.1:1234) (DC: dc)", parts.String())
}

func TestParts_Copy(t *testing.T) {
	ci.Parallel(t)

	orig := &Parts{
		Name:         "foo",
		ID:           "id",
		Region:       "region",
		Datacenter:   "dc",
		Port:         1234,
		Bootstrap:    true,
		Expect:       3,
		RaftVersion:  2,
		MajorVersion: 1,
		Build:        *version.Must(version.NewVersion("1.2.3")),
		Addr:         &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		RPCAddr:      &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5678},
		Status:       serf.StatusAlive,
		NonVoter:     true,
	}

	copied := orig.Copy()
	must.Eq(t, orig, copied)
	must.NotEq(t, fmt.Sprintf("%p", &orig), fmt.Sprintf("%p", &copied))
}

func TestIsNomadServer(t *testing.T) {
	ci.Parallel(t)

	m := serf.Member{
		Name:   "foo",
		Addr:   net.IP([]byte{127, 0, 0, 1}),
		Status: serf.StatusAlive,
		Tags: map[string]string{
			"role":     "nomad",
			"region":   "aws",
			"dc":       "east-aws",
			"rpc_addr": "1.1.1.1",
			"port":     "10000",
			"vsn":      "1",
			"raft_vsn": "2",
			"build":    "0.7.0+ent",
			"nonvoter": "1",
		},
	}

	valid, parts := IsNomadServer(m)
	must.True(t, valid)
	must.StrEqFold(t, "aws", parts.Region)
	must.StrEqFold(t, "east-aws", parts.Datacenter)
	must.Eq(t, 10000, parts.Port)
	must.StrEqFold(t, "foo", parts.Name)
	must.False(t, parts.Bootstrap)
	must.Eq(t, 0, parts.Expect)
	must.Eq(t, serf.StatusAlive, parts.Status)
	must.Eq(t, 2, parts.RaftVersion)
	must.StrEqFold(t, "1.1.1.1:10000", parts.RPCAddr.String())
	must.Eq(t, 1, parts.MajorVersion)
	must.True(t, parts.NonVoter)
	must.Eq(t, 1, parts.MajorVersion)
	must.Eq(t, 0, parts.Build.Segments()[0])
	must.Eq(t, 7, parts.Build.Segments()[1])
	must.Eq(t, 0, parts.Build.Segments()[2])
	must.True(t, parts.NonVoter)

	m.Tags["bootstrap"] = "1"
	valid, parts = IsNomadServer(m)
	must.True(t, valid)
	must.True(t, parts.Bootstrap)
	must.StrEqFold(t, "127.0.0.1:10000", parts.Addr.String())

	m.Tags["expect"] = "3"
	delete(m.Tags, "bootstrap")
	valid, parts = IsNomadServer(m)
	must.True(t, valid)
	must.Eq(t, 3, parts.Expect)

	delete(m.Tags, "nonvoter")
	valid, parts = IsNomadServer(m)
	must.True(t, valid)
	must.False(t, parts.NonVoter)
}

func TestPartsCache_PeerSet(t *testing.T) {
	ci.Parallel(t)

	peerCache := NewPeerCache()
	must.MapLen(t, 0, peerCache.peers)

	// Add an initial set of peers in the same region.
	euw1Peers := []*Parts{
		generateTestParts(t, "1.2.3", "euw1", serf.StatusAlive),
		generateTestParts(t, "1.2.3", "euw1", serf.StatusAlive),
		generateTestParts(t, "1.2.3", "euw1", serf.StatusAlive),
	}

	for _, p := range euw1Peers {
		peerCache.PeerSet(p)
	}

	must.MapLen(t, 1, peerCache.peers)
	must.Len(t, 3, peerCache.peers["euw1"])

	// Add a second set of peers in a different region.
	euw2Peers := []*Parts{
		generateTestParts(t, "1.2.3", "euw2", serf.StatusAlive),
		generateTestParts(t, "1.2.3", "euw2", serf.StatusAlive),
		generateTestParts(t, "1.2.3", "euw2", serf.StatusAlive),
	}

	for _, p := range euw2Peers {
		peerCache.PeerSet(p)
	}

	must.MapLen(t, 2, peerCache.peers)
	must.Len(t, 3, peerCache.peers["euw1"])
	must.Len(t, 3, peerCache.peers["euw2"])

	// Update a peer's status and ensure it's updated in the cache.
	changedPeer := euw2Peers[1].Copy()
	changedPeer.Status = serf.StatusFailed

	peerCache.PeerSet(changedPeer)
	must.MapLen(t, 2, peerCache.peers)
	must.Len(t, 3, peerCache.peers["euw1"])
	must.Len(t, 3, peerCache.peers["euw2"])
	must.Eq(t, serf.StatusFailed, peerCache.peers["euw2"][1].Status)
}

func TestPartsCache_PeerDelete(t *testing.T) {
	ci.Parallel(t)

	peerCache := NewPeerCache()
	must.MapLen(t, 0, peerCache.peers)

	// Add an initial set of peers in the same region.
	partsList := []*Parts{
		generateTestParts(t, "1.2.3", "euw1", serf.StatusAlive),
		generateTestParts(t, "1.2.3", "euw1", serf.StatusAlive),
		generateTestParts(t, "1.2.3", "euw1", serf.StatusAlive),
	}

	for _, p := range partsList {
		peerCache.PeerSet(p)
	}

	must.MapLen(t, 1, peerCache.peers)
	must.Len(t, 3, peerCache.peers["euw1"])

	// Create a serf.MemberEvent to delete the second peer.
	event := serf.MemberEvent{
		Members: []serf.Member{
			{
				Name:   partsList[1].Name,
				Status: serf.StatusLeft,
				Tags: map[string]string{
					"role":   "nomad",
					"region": "euw1",
					"dc":     "east-aws",
					"port":   "10000",
					"build":  "1.2.3",
					"vsn":    "1",
				},
			},
		},
	}

	peerCache.PeerDelete(event)
	must.MapLen(t, 1, peerCache.peers)
	must.Len(t, 2, peerCache.peers["euw1"])

	for _, p := range peerCache.peers["euw1"] {
		must.NotEq(t, partsList[1].Name, p.Name)
	}

	// Delete the remaining peers.
	event = serf.MemberEvent{
		Members: []serf.Member{
			{
				Name:   partsList[0].Name,
				Status: serf.StatusLeft,
				Tags: map[string]string{
					"role":   "nomad",
					"region": "euw1",
					"dc":     "east-aws",
					"port":   "10000",
					"build":  "1.2.3",
					"vsn":    "1",
				},
			},
			{
				Name:   partsList[2].Name,
				Status: serf.StatusLeft,
				Tags: map[string]string{
					"role":   "nomad",
					"region": "euw1",
					"dc":     "east-aws",
					"port":   "10000",
					"build":  "1.2.3",
					"vsn":    "1",
				},
			},
		},
	}

	peerCache.PeerDelete(event)
	must.MapLen(t, 0, peerCache.peers)
}

func TestPartsCache_ServersMeetMinimumVersion(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                    string
		inputParts              []*Parts
		inputRegion             string
		inputMinVersion         *version.Version
		inputCheckFailedServers bool
		expected                bool
	}{
		{
			name: "single peer meets version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: false,
			expected:                true,
		},
		{
			name: "single ent peer meets version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: false,
			expected:                true,
		},
		{
			name: "single dev peer meets version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.1-dev", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: false,
			expected:                true,
		},
		{
			name: "single peer fails version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.12.0")),
			inputCheckFailedServers: false,
			expected:                false,
		},
		{
			name: "single ent peer fails version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.12.0")),
			inputCheckFailedServers: false,
			expected:                false,
		},
		{
			name: "single dev peer fails version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.1-dev", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.12.0")),
			inputCheckFailedServers: false,
			expected:                false,
		},
		{
			name: "multiple peers with failed status meets version excluding failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: false,
			expected:                true,
		},
		{
			name: "multiple ent peers with failed status meets version excluding failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0+ent", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: false,
			expected:                true,
		},
		{
			name: "multiple dev peers with failed status meets version excluding failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0-dev", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0-dev", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0-dev", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: false,
			expected:                true,
		},
		{
			name: "multiple peers with failed status fails version including failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: true,
			expected:                false,
		},
		{
			name: "multiple ent peers with failed status fails version including failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0+ent", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: true,
			expected:                false,
		},
		{
			name: "multiple dev peers with failed status fails version including failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0-dev", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0-dev", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0-dev", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: true,
			expected:                false,
		},
		{
			name: "multiple mixed peers with failed status fails version including failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0-dev", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: true,
			expected:                false,
		},
		{
			name: "multiple mixed peers with failed status meets version including failed",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0-dev+ent", "euw1", serf.StatusFailed),
				generateTestParts(t, "1.11.0+ent", "euw1", serf.StatusAlive),
			},
			inputRegion:             "euw1",
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: false,
			expected:                true,
		},
		{
			name: "all regions meet minimum version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw2", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw2", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw2", serf.StatusAlive),
			},
			inputRegion:             AllRegions,
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: true,
			expected:                true,
		},
		{
			name: "all regions fails minimum version",
			inputParts: []*Parts{
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw1", serf.StatusAlive),
				generateTestParts(t, "1.10.0", "euw2", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw2", serf.StatusAlive),
				generateTestParts(t, "1.11.0", "euw2", serf.StatusAlive),
			},
			inputRegion:             AllRegions,
			inputMinVersion:         version.Must(version.NewVersion("1.11.0")),
			inputCheckFailedServers: true,
			expected:                false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			peerCache := NewPeerCache()

			for _, p := range tc.inputParts {
				peerCache.PeerSet(p)
			}

			result := peerCache.ServersMeetMinimumVersion(
				tc.inputRegion,
				tc.inputMinVersion,
				tc.inputCheckFailedServers,
			)
			must.Eq(t, tc.expected, result)
		})
	}
}

func generateTestParts(t *testing.T, version, region string, status serf.MemberStatus) *Parts {

	m := serf.Member{
		Name:   uuid.Generate(),
		Addr:   net.IP([]byte{127, 0, 0, 1}),
		Status: status,
		Tags: map[string]string{
			"role":   "nomad",
			"region": region,
			"dc":     "east-aws",
			"port":   "10000",
			"build":  version,
			"vsn":    "1",
		},
	}

	ok, parts := IsNomadServer(m)
	must.True(t, ok)

	return parts
}
