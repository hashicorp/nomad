//+build darwin

package freeport

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

/*
$ sysctl net.inet.ip.portrange.first net.inet.ip.portrange.last
net.inet.ip.portrange.first: 49152
net.inet.ip.portrange.last: 65535
*/

const (
	ephPortFirst = "net.inet.ip.portrange.first"
	ephPortLast  = "net.inet.ip.portrange.last"
	command      = "sysctl"
)

var ephPortRe = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s*$`)

func getEphemeralPortRange() (int, int, error) {
	cmd := exec.Command(command, "-n", ephPortFirst, ephPortLast)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	val := string(out)

	m := ephPortRe.FindStringSubmatch(val)
	if m != nil {
		min, err1 := strconv.Atoi(m[1])
		max, err2 := strconv.Atoi(m[2])

		if err1 == nil && err2 == nil {
			return min, max, nil
		}
	}

	return 0, 0, fmt.Errorf("unexpected sysctl value %q for keys %q %q", val, ephPortFirst, ephPortLast)
}
