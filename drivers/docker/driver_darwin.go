//go:build darwin

package docker

func setCPUSetCgroup(path string, pid int) error {
	return nil
}
