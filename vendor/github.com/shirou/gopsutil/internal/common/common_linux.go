// +build linux

package common

import (
	"os"
	"os/exec"
	"strings"
)

func DoSysctrl(mib string) ([]string, error) {
	hostEnv := os.Environ()
	foundLC := false
	for i, line := range hostEnv {
		if strings.HasPrefix(line, "LC_ALL") {
			hostEnv[i] = "LC_ALL=C"
			foundLC = true
		}
	}
	if !foundLC {
		hostEnv = append(hostEnv, "LC_ALL=C")
	}
	sysctl, err := exec.LookPath("/sbin/sysctl")
	if err != nil {
		return []string{}, err
	}
	cmd := exec.Command(sysctl, "-n", mib)
	cmd.Env = hostEnv
	out, err := cmd.Output()
	if err != nil {
		return []string{}, err
	}
	v := strings.Replace(string(out), "{ ", "", 1)
	v = strings.Replace(string(v), " }", "", 1)
	values := strings.Fields(string(v))

	return values, nil
}

func NumProcs() (uint64, error) {
	f, err := os.Open(HostProc())
	if err != nil {
		return 0, err
	}
	defer f.Close()

	list, err := f.Readdirnames(-1)
	if err != nil {
		return 0, err
	}
	return uint64(len(list)), err
}
