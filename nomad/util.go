package nomad

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
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

// RuntimeStats is used to return various runtime information
func RuntimeStats() map[string]string {
	return map[string]string{
		"kernel.name": runtime.GOOS,
		"arch":        runtime.GOARCH,
		"version":     runtime.Version(),
		"max_procs":   strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"goroutines":  strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"cpu_count":   strconv.FormatInt(int64(runtime.NumCPU()), 10),
	}
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
	RPCAddr      net.Addr
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
	expectStr, ok := m.Tags["expect"]
	var err error
	if ok {
		expect, err = strconv.Atoi(expectStr)
		if err != nil {
			return false, nil
		}
	}

	// If the server is missing the rpc_addr tag, default to the serf advertise addr
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

	raftVsn := 0
	raftVsnString, ok := m.Tags["raft_vsn"]
	if ok {
		raftVsn, err = strconv.Atoi(raftVsnString)
		if err != nil {
			return false, nil
		}
	}

	addr := &net.TCPAddr{IP: m.Addr, Port: port}
	rpcAddr := &net.TCPAddr{IP: rpcIP, Port: port}
	parts := &serverParts{
		Name:         m.Name,
		ID:           id,
		Region:       region,
		Datacenter:   datacenter,
		Port:         port,
		Bootstrap:    bootstrap,
		Expect:       expect,
		Addr:         addr,
		RPCAddr:      rpcAddr,
		MajorVersion: majorVersion,
		MinorVersion: minorVersion,
		Build:        *buildVersion,
		RaftVersion:  raftVsn,
		Status:       m.Status,
	}
	return true, parts
}

// ServersMeetMinimumVersion returns whether the given alive servers are at least on the
// given Nomad version
func ServersMeetMinimumVersion(members []serf.Member, minVersion *version.Version) bool {
	for _, member := range members {
		if valid, parts := isNomadServer(member); valid && parts.Status == serf.StatusAlive {
			// Check if the versions match - version.LessThan will return true for
			// 0.8.0-rc1 < 0.8.0, so we want to ignore the metadata
			versionsMatch := slicesMatch(minVersion.Segments(), parts.Build.Segments())
			if parts.Build.LessThan(minVersion) && !versionsMatch {
				return false
			}
		}
	}

	return true
}

func slicesMatch(a, b []int) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
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
