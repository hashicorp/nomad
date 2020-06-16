// +build darwin

package host

import (
	"bufio"
	"strings"
)

func network() string {
	return call("ifconfig")
}

func resolvConf() string {
	return call("scutil", "--dns")
}

func mountedPaths() (paths []string) {
	// TODO (langmartin) diskutil list -plist physical is a better source for disks to
	// check, but needs a little xml parsing
	out := call("mount")
	rd := bufio.NewReader(strings.NewReader(out))

	for {
		str, err := rd.ReadString('\n')
		if err != nil {
			break
		}

		// find the last ( in the line, it's the beginning of the args
		i := len(str) - 1
		for ; ; i-- {
			if str[i] == '(' {
				break
			}
		}

		// split the args part of the string
		args := strings.Split(str[i+1:len(str)-2], ", ")

		// split the device and mountpoint
		mnt := strings.Split(str[:i-1], " on ")

		switch args[0] {
		case "devfs", "autofs":
			continue
		default:
		}

		paths = append(paths, mnt[1])
	}

	return paths
}
