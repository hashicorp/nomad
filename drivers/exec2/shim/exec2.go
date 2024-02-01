// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package shim

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-set/v2"
	"github.com/hashicorp/nomad/client/anonymous"
	"github.com/hashicorp/nomad/drivers/exec2/resources"
	"github.com/hashicorp/nomad/drivers/exec2/resources/process"
	"github.com/hashicorp/nomad/helper/subproc"
	"github.com/shoenig/netlog"
	"golang.org/x/sys/unix"
)

// Options represent Task configuration options.
type Options struct {
	Command   string
	Arguments []string
	Unveil    []string
}

// Environment represents runtime configuration.
type Environment struct {
	User         string            // user the command will run as (may be empty / synthetic)
	Out          io.WriteCloser    // stdout handle
	Err          io.WriteCloser    // stderr handle
	Env          map[string]string // environment variables
	TaskDir      string            // task directory
	Cgroup       string            // task cgroup path
	Net          string            // allocation network namespace path
	Memory       uint64            // memory in megabytes
	MemoryMax    uint64            // memory_max in megabytes
	CPUBandwidth uint64            // cpu / cores bandwidth
}

type ExecTwo interface {
	// Start the Task process.
	Start(context.Context) error

	// PID returns the process ID associated with exec.
	//
	// Must only be called after Start.
	PID() int

	// Wait on the process (until exit).
	//
	// Must only be called after Start.
	Wait() error

	// Stats returns current resource utilization.
	//
	// Must only be called after Start.
	Stats() resources.Utilization

	// Signal [kill()] the process.
	//
	// Must be called after Start.
	Signal(string) error

	// Stop the process.
	//
	// Must be called after Start.
	Stop(string, time.Duration) error

	// Result of the process completion.
	//
	// Must be called after Wait.
	Result() int // exit code
}

// New an ExecTwo, an instantiation of the exec2 driver.
func New(env *Environment, opts *Options) ExecTwo {
	return &exe{
		env:  env,
		opts: opts,
		cpu:  new(resources.TrackCPU),
	}
}

// Recover an ExecTwo, an already running instance of the execc2 driver.
func Recover(pid int, env *Environment) ExecTwo {
	return &exe{
		pid:    pid,
		env:    env,
		opts:   nil, // already started, no use
		waiter: process.WaitOnOrphan(pid),
		signal: process.Interrupts(pid),
		cpu:    new(resources.TrackCPU),
	}
}

type exe struct {
	// comes from task config
	env  *Environment
	opts *Options

	// comes from runtime
	pid    int
	cpu    *resources.TrackCPU
	waiter process.Waiter
	signal process.Signaler
	code   int
}

func (e *exe) Start(ctx context.Context) error {
	uid, gid, home, err := anonymous.LookupUser(e.env.User)
	if err != nil {
		return fmt.Errorf("failed to lookup user: %w", err)
	}

	// find out cgroup file descriptor
	fd, cleanup, err := e.openCG()
	if err != nil {
		return fmt.Errorf("failed to open cgroup for descriptor: %w", err)
	}

	// set resource constraints
	if err = e.constrain(); err != nil {
		return fmt.Errorf("failed to write cgroup constraints: %w", err)
	}

	// create sandbox using nsenter, unshare, and our cgroup
	// TODO make use of landlock
	cmd := e.isolation(ctx, home, fd, uid, gid)
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// close the cgroup descriptor after start
	// avoid using defer because we do not need this open during Wait()
	cleanup()

	// attach to the underlying unix process
	e.pid = cmd.Process.Pid
	e.waiter = process.WaitOnChild(cmd.Process)
	e.signal = process.Interrupts(cmd.Process.Pid)

	return nil
}

func (e *exe) PID() int {
	return e.pid
}

func (e *exe) Wait() error {
	exit := e.waiter.Wait()
	e.code = exit.Code
	return exit.Err
}

func (e *exe) Signal(s string) error {
	return e.signal.Signal(s)
}

func (e *exe) Stop(signal string, timeout time.Duration) error {
	// politely ask the group to terminate via user specified signal
	err := e.Signal(signal)
	if e.blockPIDs(timeout) {
		// no more mr. nice guy, kill the whole cgroup
		_ = e.writeCG("cgroup.kill", "1")
		_ = e.env.Out.Close()
		_ = e.env.Err.Close()
	}
	return err
}

func (e *exe) Result() int {
	return e.code
}

func (e *exe) Stats() resources.Utilization {
	memCurrentS, _ := e.readCG("memory.current")
	memCurrent, _ := strconv.Atoi(memCurrentS)

	swapCurrentS, _ := e.readCG("memory.swap.current")
	swapCurrent, _ := strconv.Atoi(swapCurrentS)

	memStatS, _ := e.readCG("memory.stat")
	memCache := extractRe(memStatS, memCacheRe)

	cpuStatsS, _ := e.readCG("cpu.stat")
	usr, system, total := extractCPU(cpuStatsS)
	userPct, systemPct, totalPct := e.cpu.Percent(usr, system, total)

	specs, _ := resources.Get()
	ticks := (.01 * totalPct) * resources.Percent(specs.Ticks()/specs.Cores)

	return resources.Utilization{
		// memory stats
		Memory: uint64(memCurrent),
		Swap:   uint64(swapCurrent),
		Cache:  memCache,

		// cpu stats
		System:  systemPct,
		User:    userPct,
		Percent: totalPct,
		Ticks:   ticks,
	}
}

func (e *exe) openCG() (int, func(), error) {
	fd, err := unix.Open(e.env.Cgroup, unix.O_PATH, 0)
	cleanup := func() { _ = unix.Close(fd) }
	return fd, cleanup, err
}

func (e *exe) readCG(file string) (string, error) {
	file = filepath.Join(e.env.Cgroup, file)
	b, err := os.ReadFile(file)
	return strings.TrimSpace(string(b)), err
}

func (e *exe) writeCG(file, content string) error {
	file = filepath.Join(e.env.Cgroup, file)
	f, err := os.OpenFile(file, os.O_WRONLY, 0o700)
	if err != nil {
		return fmt.Errorf("failed to open cgroup file: %w", err)
	}
	if _, err = io.Copy(f, strings.NewReader(content)); err != nil {
		return fmt.Errorf("failed to write pid to cgroup file: %w", err)
	}
	return f.Close()
}

func flatten(user, home string, env map[string]string) []string {
	result := make([]string, 0, len(env))

	// override and remove some variables
	useless := set.From([]string{"LS_COLORS", "XAUTHORITY", "DISPLAY", "COLORTERM", "MAIL"})
	env["USER"] = user
	env["HOME"] = home

	// set the tmp directory to the one made for the task
	parent := filepath.Dir(env["NOMAD_TASK_DIR"])
	tmp := filepath.Join(parent, "tmp")
	env["TMPDIR"] = tmp

	for k, v := range env {
		switch {
		case useless.Contains(k): // purge useless vars
			continue
		case v == "":
			result = append(result, k)
		default:
			result = append(result, k+"="+v)
		}
	}

	return result
}

func (e *exe) parameters(uid, gid int) []string {
	var result []string

	// setup nsenter if using bridge mode
	if net := e.env.Net; net != "" {
		result = append(
			result,
			"nsenter",
			"--no-fork",
			fmt.Sprintf("--net=%s", net),
			"--",
		)
	}

	// setup unshare for ipc, pid namespaces
	result = append(result,
		"unshare",
		"--ipc",
		"--pid",
		"--mount-proc",
		"--fork",
		"--kill-child=SIGKILL",
		"--setuid", strconv.Itoa(uid),
		"--setgid", strconv.Itoa(gid),
		"--",
	)

	// TODO remove
	e.opts.Unveil = []string{"/etc/passwd:r", "/usr/bin:rx"}

	// setup ourself 'nomad exec2-shim' for unveil
	result = append(result, subproc.Self(), SubCommand)
	result = append(result, e.opts.Unveil...)
	result = append(result, "--")

	// append the user command
	result = append(result, e.opts.Command)
	if len(e.opts.Arguments) > 0 {
		result = append(result, e.opts.Arguments...)
	}

	// craft complete result
	return result
}

// setup the process to be run
func (e *exe) isolation(ctx context.Context, home string, fd, uid, gid int) *exec.Cmd {
	params := e.parameters(uid, gid)
	netlog.Blue("exe", "params", params)
	cmd := exec.CommandContext(ctx, params[0], params[1:]...)
	cmd.Stdout = e.env.Out
	cmd.Stderr = e.env.Err
	cmd.Env = flatten(e.env.User, home, e.env.Env)
	cmd.Dir = e.env.TaskDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		UseCgroupFD: true, // clone directly into cgroup
		CgroupFD:    fd,   // cgroup file descriptor
		Setpgid:     true, // ignore signals sent to nomad
	}
	return cmd
}

// set resource constraints via cgroups
func (e *exe) constrain() error {
	// set cpu bandwidth
	_ = e.writeCG("cpu.max", fmt.Sprintf("%d 100000", e.env.CPUBandwidth))

	// will want to set burst one day, but in coordination with nomad

	// set memory limits
	switch e.env.MemoryMax {
	case 0:
		_ = e.writeCG("memory.max", fmt.Sprintf("%d", e.env.Memory))
	default:
		_ = e.writeCG("memory.low", fmt.Sprintf("%d", e.env.Memory))
		_ = e.writeCG("memory.max", fmt.Sprintf("%d", e.env.MemoryMax))
	}

	return nil
}

var (
	memCacheRe = regexp.MustCompile(`file\s+(\d+)`)
)

func extractRe(s string, re *regexp.Regexp) uint64 {
	matches := memCacheRe.FindStringSubmatch(s)
	if len(matches) != 2 {
		return 0
	}
	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}
	return uint64(value)
}

func extractCPU(s string) (user, system, total resources.MicroSecond) {
	read := func(line string, i *resources.MicroSecond) {
		num := line[strings.Index(line, " ")+1:]
		v, _ := strconv.ParseInt(num, 10, 64)
		*i = resources.MicroSecond(v)
	}
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		text := scanner.Text()
		switch {
		case strings.HasPrefix(text, "user_usec"):
			read(text, &user)
		case strings.HasPrefix(text, "system_usec"):
			read(text, &system)
		case strings.HasPrefix(text, "usage_usec"):
			read(text, &total)
		}
	}
	return
}

// blockPIDs blocks until there are no more live processes in the cgroup, and returns true
// if the timeout is exceeded or an error occurs.
func (e *exe) blockPIDs(timeout time.Duration) bool {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	abort := time.After(timeout)

	for {
		select {
		case <-ticker.C:
			count := e.currentPIDs()
			switch count {
			case 0:
				// processes are no longer running
				return false
			case -1:
				// failed to read cgroups file, issue force kill
				return true
			default:
				// processes are still running, wait longer
			}
		case <-abort:
			// timeout exceeded, issue force kill
			return true
		}
	}
}

// currentPIDs returns the number of live processes in the cgroup.
func (e *exe) currentPIDs() int {
	s, err := e.readCG("pids.current")
	if err != nil {
		return -1
	}
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return i
}
