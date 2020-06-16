// +build linux

package host

import (
	"bufio"
	"strings"
)

func network() string {
	return call("ip", "address", "list")
}

func resolvConf() string {
	return slurp("/etc/resolv.conf")
}

// mountedPaths produces a list of mounts
func mountedPaths() []string {
	out := call("findmnt", "-D", "-tnotmpfs,nodevtmpfs", "-o", "TARGET")
	rd := bufio.NewReader(strings.NewReader(out))

	var paths []string
	for {
		str, err := rd.ReadString('\n')
		if err != nil {
			break
		}
		paths = append(paths, str)
	}

	return paths
}
