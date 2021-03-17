// +build darwin dragonfly freebsd netbsd openbsd solaris windows

package fingerprint

import "fmt"

func deriveCpuset() ([]uint32, error) {
	return nil, fmt.Errorf("not implemented for this platform")
}
