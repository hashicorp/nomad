// +build freebsd

package host

import (
	"syscall"
)

// IntToString converts an integer to a string
func IntToString(orig []int8) string {
	ret := make([]byte, len(orig))
	size := -1
	for i, o := range orig {
		if o == 0 {
			size = i
			break
		}
		ret[i] = byte(o)
	}
	if size == -1 {
		size = len(orig)
	}

	return string(ret[0:size])
}

// mountedPaths produces a list of mounts
func mountedPaths() []string {
	var paths []string

	n, err := syscall.Getfsstat(nil, 1)
	if err != nil {
		return []string{err.Error()}
	}

	data := make([]syscall.Statfs_t, n)
	_, err = syscall.Getfsstat(data, 1)
	if err != nil {
		return []string{err.Error()}
	}

	empty := syscall.Statfs_t{}
	for _, stat := range data {
		if stat == empty {
			return []string{"empy stat struct returned"}
		}
		paths = append(paths, intToString(stat.Mntonname[:]))
	}

	return paths
}
