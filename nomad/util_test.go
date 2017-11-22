package nomad

import (
	"errors"
	"net"
	"reflect"
	"testing"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/serf/serf"
)

func TestIsNomadServer(t *testing.T) {
	t.Parallel()
	m := serf.Member{
		Name:   "foo",
		Addr:   net.IP([]byte{127, 0, 0, 1}),
		Status: serf.StatusAlive,
		Tags: map[string]string{
			"role":   "nomad",
			"region": "aws",
			"dc":     "east-aws",
			"port":   "10000",
			"vsn":    "1",
			"build":  "0.7.0+ent",
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
	if seg := parts.Build.Segments(); len(seg) != 3 {
		t.Fatalf("bad: %v", parts.Build)
	} else if seg[0] != 0 && seg[1] != 7 && seg[2] != 0 {
		t.Fatalf("bad: %v", parts.Build)
	}

	m.Tags["bootstrap"] = "1"
	valid, parts = isNomadServer(m)
	if !valid || !parts.Bootstrap {
		t.Fatalf("expected bootstrap")
	}
	if parts.Addr.String() != "127.0.0.1:10000" {
		t.Fatalf("bad addr: %v", parts.Addr)
	}
	if parts.MajorVersion != 1 {
		t.Fatalf("bad: %v", parts)
	}

	m.Tags["expect"] = "3"
	delete(m.Tags, "bootstrap")
	valid, parts = isNomadServer(m)
	if !valid || parts.Expect != 3 {
		t.Fatalf("bad: %v", parts.Expect)
	}
}

func TestServersMeetMinimumVersion(t *testing.T) {
	t.Parallel()
	makeMember := func(version string) serf.Member {
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
			Status: serf.StatusAlive,
		}
	}

	cases := []struct {
		members  []serf.Member
		ver      *version.Version
		expected bool
	}{
		// One server, meets reqs
		{
			members: []serf.Member{
				makeMember("0.7.5"),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// One server in dev, meets reqs
		{
			members: []serf.Member{
				makeMember("0.8.5-dev"),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// One server with meta, meets reqs
		{
			members: []serf.Member{
				makeMember("0.7.5+ent"),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// One server, doesn't meet reqs
		{
			members: []serf.Member{
				makeMember("0.7.5"),
			},
			ver:      version.Must(version.NewVersion("0.8.0")),
			expected: false,
		},
		// Multiple servers, meets req version
		{
			members: []serf.Member{
				makeMember("0.7.5"),
				makeMember("0.8.0"),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// Multiple servers, doesn't meet req version
		{
			members: []serf.Member{
				makeMember("0.7.5"),
				makeMember("0.8.0"),
			},
			ver:      version.Must(version.NewVersion("0.8.0")),
			expected: false,
		},
	}

	for _, tc := range cases {
		result := ServersMeetMinimumVersion(tc.members, tc.ver)
		if result != tc.expected {
			t.Fatalf("bad: %v, %v, %v", result, tc.ver.String(), tc)
		}
	}
}

func TestMinRaftProtocol(t *testing.T) {
	t.Parallel()
	makeMember := func(version, datacenter string) serf.Member {
		return serf.Member{
			Name: "foo",
			Addr: net.IP([]byte{127, 0, 0, 1}),
			Tags: map[string]string{
				"role":     "nomad",
				"region":   "aws",
				"dc":       datacenter,
				"port":     "10000",
				"vsn":      "1",
				"raft_vsn": version,
			},
			Status: serf.StatusAlive,
		}
	}

	cases := []struct {
		members    []serf.Member
		datacenter string
		expected   int
		err        error
	}{
		// No servers, error
		{
			members:  []serf.Member{},
			expected: -1,
			err:      errors.New("no servers found"),
		},
		// One server
		{
			members: []serf.Member{
				makeMember("1", "dc1"),
			},
			datacenter: "dc1",
			expected:   1,
		},
		// One server, bad version formatting
		{
			members: []serf.Member{
				makeMember("asdf", "dc1"),
			},
			datacenter: "dc1",
			expected:   -1,
			err:        errors.New(`strconv.Atoi: parsing "asdf": invalid syntax`),
		},
		// One server, wrong datacenter
		{
			members: []serf.Member{
				makeMember("1", "dc1"),
			},
			datacenter: "dc2",
			expected:   -1,
			err:        errors.New("no servers found"),
		},
		// Multiple servers, different versions
		{
			members: []serf.Member{
				makeMember("1", "dc1"),
				makeMember("2", "dc1"),
			},
			datacenter: "dc1",
			expected:   1,
		},
		// Multiple servers, same version
		{
			members: []serf.Member{
				makeMember("2", "dc1"),
				makeMember("2", "dc1"),
			},
			datacenter: "dc1",
			expected:   2,
		},
		// Multiple servers, multiple datacenters
		{
			members: []serf.Member{
				makeMember("3", "dc1"),
				makeMember("2", "dc1"),
				makeMember("1", "dc2"),
			},
			datacenter: "dc1",
			expected:   2,
		},
	}

	for _, tc := range cases {
		result, err := MinRaftProtocol(tc.datacenter, tc.members)
		if result != tc.expected {
			t.Fatalf("bad: %v, %v, %v", result, tc.expected, tc)
		}
		if tc.err != nil {
			if err == nil || tc.err.Error() != err.Error() {
				t.Fatalf("bad: %v, %v, %v", err, tc.err, tc)
			}
		}
	}
}

func TestShuffleStrings(t *testing.T) {
	t.Parallel()
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

func TestMaxUint64(t *testing.T) {
	t.Parallel()
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
