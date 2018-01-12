package nomad

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

// ensurePath is used to make sure a path exists
func ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

// serverParts is used to return the parts of a server role
type serverParts struct {
	Name         string
	ID           string
	Region       string
	Datacenter   string
	Port         int
	Bootstrap    bool
	Expect       int
	MajorVersion int
	MinorVersion int
	Build        version.Version
	RaftVersion  int
	Addr         net.Addr
	Status       serf.MemberStatus
}

func (s *serverParts) String() string {
	return fmt.Sprintf("%s (Addr: %s) (DC: %s)",
		s.Name, s.Addr, s.Datacenter)
}

// Returns if a member is a Nomad server. Returns a boolean,
// and a struct with the various important components
func isNomadServer(m serf.Member) (bool, *serverParts) {
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
	expect_str, ok := m.Tags["expect"]
	var err error
	if ok {
		expect, err = strconv.Atoi(expect_str)
		if err != nil {
			return false, nil
		}
	}

	port_str := m.Tags["port"]
	port, err := strconv.Atoi(port_str)
	if err != nil {
		return false, nil
	}

	build_version, err := version.NewVersion(m.Tags["build"])
	if err != nil {
		return false, nil
	}

	// The "vsn" tag was Version, which is now the MajorVersion number.
	majorVersionStr := m.Tags["vsn"]
	majorVersion, err := strconv.Atoi(majorVersionStr)
	if err != nil {
		return false, nil
	}

	// To keep some semblance of convention, "mvn" is now the "Minor
	// Version Number."
	minorVersionStr := m.Tags["mvn"]
	minorVersion, err := strconv.Atoi(minorVersionStr)
	if err != nil {
		minorVersion = 0
	}

	raft_vsn := 0
	raft_vsn_str, ok := m.Tags["raft_vsn"]
	if ok {
		raft_vsn, err = strconv.Atoi(raft_vsn_str)
		if err != nil {
			return false, nil
		}
	}

	addr := &net.TCPAddr{IP: m.Addr, Port: port}
	parts := &serverParts{
		Name:         m.Name,
		ID:           id,
		Region:       region,
		Datacenter:   datacenter,
		Port:         port,
		Bootstrap:    bootstrap,
		Expect:       expect,
		Addr:         addr,
		MajorVersion: majorVersion,
		MinorVersion: minorVersion,
		Build:        *build_version,
		RaftVersion:  raft_vsn,
		Status:       m.Status,
	}
	return true, parts
}

// ServersMeetMinimumVersion returns whether the given alive servers are at least on the
// given Nomad version
func ServersMeetMinimumVersion(members []serf.Member, minVersion *version.Version) bool {
	for _, member := range members {
		if valid, parts := isNomadServer(member); valid && parts.Status == serf.StatusAlive {
			if parts.Build.LessThan(minVersion) {
				return false
			}
		}
	}

	return true
}

// MinRaftProtocol returns the lowest supported Raft protocol among alive servers
// in the given region.
func MinRaftProtocol(region string, members []serf.Member) (int, error) {
	minVersion := -1
	for _, m := range members {
		if m.Tags["role"] != "nomad" || m.Tags["region"] != region || m.Status != serf.StatusAlive {
			continue
		}

		vsn, ok := m.Tags["raft_vsn"]
		if !ok {
			vsn = "1"
		}
		raftVsn, err := strconv.Atoi(vsn)
		if err != nil {
			return -1, err
		}

		if minVersion == -1 || raftVsn < minVersion {
			minVersion = raftVsn
		}
	}

	if minVersion == -1 {
		return minVersion, fmt.Errorf("no servers found")
	}

	return minVersion, nil
}

// shuffleStrings randomly shuffles the list of strings
func shuffleStrings(list []string) {
	for i := range list {
		j := rand.Intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}

// maxUint64 returns the maximum value
func maxUint64(inputs ...uint64) uint64 {
	l := len(inputs)
	if l == 0 {
		return 0
	} else if l == 1 {
		return inputs[0]
	}

	max := inputs[0]
	for i := 1; i < l; i++ {
		cur := inputs[i]
		if cur > max {
			max = cur
		}
	}
	return max
}
