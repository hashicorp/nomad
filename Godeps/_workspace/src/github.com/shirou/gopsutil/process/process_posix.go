// +build linux freebsd darwin

package process

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// POSIX
func getTerminalMap() (map[uint64]string, error) {
	ret := make(map[uint64]string)
	var termfiles []string

	d, err := os.Open("/dev")
	if err != nil {
		return nil, err
	}
	defer d.Close()

	devnames, err := d.Readdirnames(-1)
	for _, devname := range devnames {
		if strings.HasPrefix(devname, "/dev/tty") {
			termfiles = append(termfiles, "/dev/tty/"+devname)
		}
	}

	ptsd, err := os.Open("/dev/pts")
	if err != nil {
		return nil, err
	}
	defer ptsd.Close()

	ptsnames, err := ptsd.Readdirnames(-1)
	for _, ptsname := range ptsnames {
		termfiles = append(termfiles, "/dev/pts/"+ptsname)
	}

	for _, name := range termfiles {
		stat := syscall.Stat_t{}
		if err = syscall.Stat(name, &stat); err != nil {
			return nil, err
		}
		rdev := uint64(stat.Rdev)
		ret[rdev] = strings.Replace(name, "/dev", "", -1)
	}
	return ret, nil
}

func (p *Process) SendSignal(sig syscall.Signal) error {
	sigAsStr := "INT"
	switch sig {
	case syscall.SIGSTOP:
		sigAsStr = "STOP"
	case syscall.SIGCONT:
		sigAsStr = "CONT"
	case syscall.SIGTERM:
		sigAsStr = "TERM"
	case syscall.SIGKILL:
		sigAsStr = "KILL"
	}

	cmd := exec.Command("kill", "-s", sigAsStr, strconv.Itoa(int(p.Pid)))
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (p *Process) Suspend() error {
	return p.SendSignal(syscall.SIGSTOP)
}
func (p *Process) Resume() error {
	return p.SendSignal(syscall.SIGCONT)
}
func (p *Process) Terminate() error {
	return p.SendSignal(syscall.SIGTERM)
}
func (p *Process) Kill() error {
	return p.SendSignal(syscall.SIGKILL)
}
func (p *Process) Username() (string, error) {
	uids, err := p.Uids()
	if err != nil {
		return "", err
	}
	if len(uids) > 0 {
		u, err := user.LookupId(strconv.Itoa(int(uids[0])))
		if err != nil {
			return "", err
		}
		return u.Username, nil
	}
	return "", nil
}
