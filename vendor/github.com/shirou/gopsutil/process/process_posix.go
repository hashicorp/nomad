// +build linux freebsd openbsd darwin

package process

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
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

	var ptsnames []string
	ptsd, err := os.Open("/dev/pts")
	if err != nil {
		ptsnames, _ = filepath.Glob("/dev/ttyp*")
		if ptsnames == nil {
			return nil, err
		}
	}
	defer ptsd.Close()

	if ptsnames == nil {
		defer ptsd.Close()
		ptsnames, err = ptsd.Readdirnames(-1)
		for _, ptsname := range ptsnames {
			termfiles = append(termfiles, "/dev/pts/"+ptsname)
		}
	} else {
		termfiles = ptsnames
	}

	for _, name := range termfiles {
		stat := unix.Stat_t{}
		if err = unix.Stat(name, &stat); err != nil {
			return nil, err
		}
		rdev := uint64(stat.Rdev)
		ret[rdev] = strings.Replace(name, "/dev", "", -1)
	}
	return ret, nil
}

// SendSignal sends a unix.Signal to the process.
// Currently, SIGSTOP, SIGCONT, SIGTERM and SIGKILL are supported.
func (p *Process) SendSignal(sig syscall.Signal) error {
	process, err := os.FindProcess(int(p.Pid))
	if err != nil {
		return err
	}

	err = process.Signal(sig)
	if err != nil {
		return err
	}

	return nil
}

// Suspend sends SIGSTOP to the process.
func (p *Process) Suspend() error {
	return p.SendSignal(unix.SIGSTOP)
}

// Resume sends SIGCONT to the process.
func (p *Process) Resume() error {
	return p.SendSignal(unix.SIGCONT)
}

// Terminate sends SIGTERM to the process.
func (p *Process) Terminate() error {
	return p.SendSignal(unix.SIGTERM)
}

// Kill sends SIGKILL to the process.
func (p *Process) Kill() error {
	return p.SendSignal(unix.SIGKILL)
}

// Username returns a username of the process.
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
