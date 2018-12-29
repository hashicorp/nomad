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
	Region       string
	Datacenter   string
	Port         int
	Bootstrap    bool
	Expect       int
	MajorVersion int
	MinorVersion int
	Build        version.Version
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

	addr := &net.TCPAddr{IP: m.Addr, Port: port}
	parts := &serverParts{
		Name:         m.Name,
		Region:       region,
		Datacenter:   datacenter,
		Port:         port,
		Bootstrap:    bootstrap,
		Expect:       expect,
		Addr:         addr,
		MajorVersion: majorVersion,
		MinorVersion: minorVersion,
		Build:        *build_version,
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
