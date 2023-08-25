// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"net"
	"reflect"
	"testing"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

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
	valid, parts := isNomadServer(m)
	if !valid || parts.Region != "aws" ||
		parts.Datacenter != "east-aws" || parts.Port != 10000 {
		t.Fatalf("bad: %v %v", valid, parts)
	}
	if parts.Name != "foo" {
		t.Fatalf("bad: %v", parts)
	}
	if parts.Bootstrap {
		t.Fatalf("unexpected bootstrap")
	}
	if parts.Expect != 0 {
		t.Fatalf("bad: %v", parts.Expect)
	}
	if parts.Status != serf.StatusAlive {
		t.Fatalf("bad: %v", parts.Status)
	}
	if parts.RaftVersion != 2 {
		t.Fatalf("bad: %v", parts.RaftVersion)
	}
	if parts.RPCAddr.String() != "1.1.1.1:10000" {
		t.Fatalf("bad: %v", parts.RPCAddr.String())
	}
	require.Equal(t, 1, parts.MajorVersion)
	if seg := parts.Build.Segments(); len(seg) != 3 {
		t.Fatalf("bad: %v", parts.Build)
	} else if seg[0] != 0 && seg[1] != 7 && seg[2] != 0 {
		t.Fatalf("bad: %v", parts.Build)
	}
	if !parts.NonVoter {
		t.Fatalf("should be nonvoter")
	}

	m.Tags["bootstrap"] = "1"
	valid, parts = isNomadServer(m)
	if !valid || !parts.Bootstrap {
		t.Fatalf("expected bootstrap")
	}
	if parts.Addr.String() != "127.0.0.1:10000" {
		t.Fatalf("bad addr: %v", parts.Addr)
	}

	m.Tags["expect"] = "3"
	delete(m.Tags, "bootstrap")
	valid, parts = isNomadServer(m)
	if !valid || parts.Expect != 3 {
		t.Fatalf("bad: %v", parts.Expect)
	}

	delete(m.Tags, "nonvoter")
	valid, parts = isNomadServer(m)
	if !valid || parts.NonVoter {
		t.Fatalf("should be a voter")
	}
}

func TestServersMeetMinimumVersionExcludingFailed(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		members  []serf.Member
		ver      *version.Version
		expected bool
	}{
		// One server, meets reqs
		{
			members: []serf.Member{
				makeMember("0.7.5", serf.StatusAlive),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// One server in dev, meets reqs
		{
			members: []serf.Member{
				makeMember("0.8.5-dev", serf.StatusAlive),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// One server with meta, meets reqs
		{
			members: []serf.Member{
				makeMember("0.7.5+ent", serf.StatusAlive),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// One server, doesn't meet reqs
		{
			members: []serf.Member{
				makeMember("0.7.5", serf.StatusAlive),
			},
			ver:      version.Must(version.NewVersion("0.8.0")),
			expected: false,
		},
		// Multiple servers, meets req version, includes failed that doesn't meet req
		{
			members: []serf.Member{
				makeMember("0.7.5", serf.StatusAlive),
				makeMember("0.8.0", serf.StatusAlive),
				makeMember("0.7.0", serf.StatusFailed),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// Multiple servers, doesn't meet req version
		{
			members: []serf.Member{
				makeMember("0.7.5", serf.StatusAlive),
				makeMember("0.8.0", serf.StatusAlive),
			},
			ver:      version.Must(version.NewVersion("0.8.0")),
			expected: false,
		},
	}

	for _, tc := range cases {
		result := ServersMeetMinimumVersion(tc.members, AllRegions, tc.ver, false)
		if result != tc.expected {
			t.Fatalf("bad: %v, %v, %v", result, tc.ver.String(), tc)
		}
	}
}

func TestServersMeetMinimumVersionIncludingFailed(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		members  []serf.Member
		ver      *version.Version
		expected bool
	}{
		// Multiple servers, meets req version
		{
			members: []serf.Member{
				makeMember("0.7.5", serf.StatusAlive),
				makeMember("0.8.0", serf.StatusAlive),
				makeMember("0.7.5", serf.StatusFailed),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// Multiple servers, doesn't meet req version
		{
			members: []serf.Member{
				makeMember("0.7.5", serf.StatusAlive),
				makeMember("0.8.0", serf.StatusAlive),
				makeMember("0.7.0", serf.StatusFailed),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: false,
		},
	}

	for _, tc := range cases {
		result := ServersMeetMinimumVersion(tc.members, AllRegions, tc.ver, true)
		if result != tc.expected {
			t.Fatalf("bad: %v, %v, %v", result, tc.ver.String(), tc)
		}
	}
}

func TestServersMeetMinimumVersionSuffix(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		members  []serf.Member
		ver      *version.Version
		expected bool
	}{
		// Multiple servers, meets req version
		{
			members: []serf.Member{
				makeMember("1.3.0", serf.StatusAlive),
				makeMember("1.2.6", serf.StatusAlive),
				makeMember("1.2.6-dev", serf.StatusFailed),
			},
			ver:      version.Must(version.NewVersion("1.2.6-dev")),
			expected: true,
		},
		// Multiple servers, doesn't meet req version
		{
			members: []serf.Member{
				makeMember("1.1.18", serf.StatusAlive),
				makeMember("1.2.6-dev", serf.StatusAlive),
				makeMember("1.0.11", serf.StatusFailed),
			},
			ver:      version.Must(version.NewVersion("1.2.6-dev")),
			expected: false,
		},
	}

	for _, tc := range cases {
		result := ServersMeetMinimumVersion(tc.members, AllRegions, tc.ver, true)
		if result != tc.expected {
			t.Fatalf("bad: %v, %v, %v", result, tc.ver.String(), tc)
		}
	}
}

func makeMember(version string, status serf.MemberStatus) serf.Member {
	return serf.Member{
		Name: "foo",
		Addr: net.IP([]byte{127, 0, 0, 1}),
		Tags: map[string]string{
			"role":   "nomad",
			"region": "aws",
			"dc":     "east-aws",
			"port":   "10000",
			"build":  version,
			"vsn":    "1",
		},
		Status: status,
	}
}

func TestShuffleStrings(t *testing.T) {
	ci.Parallel(t)
	// Generate input
	inp := make([]string, 10)
	for idx := range inp {
		inp[idx] = uuid.Generate()
	}

	// Copy the input
	orig := make([]string, len(inp))
	copy(orig, inp)

	// Shuffle
	shuffleStrings(inp)

	// Ensure order is not the same
	if reflect.DeepEqual(inp, orig) {
		t.Fatalf("shuffle failed")
	}
}

func Test_partitionAll(t *testing.T) {
	xs := []string{"a", "b", "c", "d", "e", "f"}
	// evenly divisible
	require.Equal(t, [][]string{{"a", "b"}, {"c", "d"}, {"e", "f"}}, partitionAll(2, xs))
	require.Equal(t, [][]string{{"a", "b", "c"}, {"d", "e", "f"}}, partitionAll(3, xs))
	// whole thing fits int the last part
	require.Equal(t, [][]string{{"a", "b", "c", "d", "e", "f"}}, partitionAll(7, xs))
	// odd remainder
	require.Equal(t, [][]string{{"a", "b", "c", "d"}, {"e", "f"}}, partitionAll(4, xs))
	// zero size
	require.Equal(t, [][]string{{"a", "b", "c", "d", "e", "f"}}, partitionAll(0, xs))
	// one size
	require.Equal(t, [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}}, partitionAll(1, xs))
}

func TestMaxUint64(t *testing.T) {
	ci.Parallel(t)
	if maxUint64(1, 2) != 2 {
		t.Fatalf("bad")
	}
	if maxUint64(2, 2) != 2 {
		t.Fatalf("bad")
	}
	if maxUint64(2, 1) != 2 {
		t.Fatalf("bad")
	}
}
