// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package executor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/drivers/shared/executor/s4u"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/sys/windows"
)

func NewExecutorWithIsolation(logger hclog.Logger, compute cpustats.Compute) Executor {
	logger = logger.Named("executor")
	logger.Error("isolation executor is not supported on this platform, using default")
	return NewExecutor(logger, compute)
}

func (e *UniversalExecutor) configureResourceContainer(_ *ExecCommand, _ int) (func() error, func(), error) {
	cleanup := func() {}
	running := func() error { return nil }
	return running, cleanup, nil
}

func (e *UniversalExecutor) start(command *ExecCommand) error {
	return e.childCmd.Start()
}

func withNetworkIsolation(f func() error, _ *drivers.NetworkIsolationSpec) error {
	return f()
}

func setCmdUser(logger hclog.Logger, cmd *exec.Cmd, user string) error {
	token, err := createUserToken(logger, user)
	if err != nil {
		return fmt.Errorf("failed to create user token: %w", err)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Token = token

	runtime.AddCleanup(cmd, func(attr *syscall.SysProcAttr) {
		_ = attr.Token.Close()
	}, cmd.SysProcAttr)

	return nil
}

var (
	advapiDll      = windows.NewLazySystemDLL("advapi32.dll")
	procLogonUserW = advapiDll.NewProc("LogonUserW")
)

const (
	_LOGON_SERVICE    uint32 = 5
	_PROVIDER_DEFAULT uint32 = 0
)

// username can be of the form "domain\username", ".\username" or "username@domain"
func createUserToken(logger hclog.Logger, username string) (syscall.Token, error) {
	var token windows.Token
	var err error

	var runAsUpn string
	if strings.IndexByte(username, '\\') != -1 {
		runAsUpn, err = convertUserToUpn(username)
		if err != nil {
			return 0, fmt.Errorf("failed to convert username %q to UPN : %w", username, err)
		}
	} else if strings.IndexByte(username, '@') != -1 {
		runAsUpn = username
	}

	logger.Debug("creating user token", "username", username, "runAsUpn", runAsUpn)

	if runAsUpn != "" {
		token, err = s4u.GetDomainS4uToken(runAsUpn)
	} else {
		token, err = s4u.GetLocalS4uToken(username)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to create S4U token for user : %w", err)
	}

	return syscall.Token(token), nil
}

func convertUserToUpn(username string) (string, error) {
	usernameUtf16, err := windows.UTF16FromString(username)
	if err != nil {
		return "", fmt.Errorf("error converting username to UTF16 : %w", err)
	}

	upnUtf16, err := translateSamToUpn(usernameUtf16)
	if err != nil {
		return "", err
	}

	return windows.UTF16ToString(upnUtf16), nil
}

const MAX_UPN_LEN = 1024

func translateSamToUpn(samAccountNameUtf16 []uint16) ([]uint16, error) {
	var domainUpnLen uint32 = MAX_UPN_LEN + 1
	domainUpn := make([]uint16, domainUpnLen)
	err := windows.TranslateName(&samAccountNameUtf16[0], windows.NameSamCompatible, windows.NameUserPrincipal, &domainUpn[0], &domainUpnLen)
	if err != nil {
		return nil, err
	}
	return domainUpn[:domainUpnLen-1], nil
}

func createUserTokenOld(username, domain string) (*syscall.Token, error) {
	userw, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return nil, fmt.Errorf("failed to convert username to UTF-16: %w", err)
	}
	domainw, err := syscall.UTF16PtrFromString(domain)
	if err != nil {
		return nil, fmt.Errorf("failed to convert user domain to UTF-16: %w", err)
	}
	var token syscall.Token
	ret, _, e := procLogonUserW.Call(
		uintptr(unsafe.Pointer(userw)),
		uintptr(unsafe.Pointer(domainw)),
		uintptr(unsafe.Pointer(nil)),
		uintptr(_LOGON_SERVICE),
		uintptr(_PROVIDER_DEFAULT),
		uintptr(unsafe.Pointer(&token)),
	)
	if ret == 0 {
		return nil, e
	}

	return &token, nil
}

func (e *UniversalExecutor) ListProcesses() set.Collection[int] {
	return procstats.ListByPid(e.childCmd.Process.Pid)
}

func (e *UniversalExecutor) setSubCmdCgroup(*exec.Cmd, string) (func(), error) {
	return func() {}, nil
}

// configure new process group for child process and creates a JobObject for the
// executor. Children of the executor will be created in the same JobObject
// Ref: https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
func (e *UniversalExecutor) setNewProcessGroup() error {
	// We need to check that as build flags includes windows for this file
	if e.childCmd.SysProcAttr == nil {
		e.childCmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.childCmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP

	// note: we don't call CloseHandle on this job handle because we need to
	// hold onto it until the executor exits
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("could not create Windows job object for executor: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)))
	if err != nil {
		return fmt.Errorf("could not configure Windows job object for executor: %w", err)
	}

	handle := windows.CurrentProcess()
	err = windows.AssignProcessToJobObject(job, handle)
	if err != nil {
		return fmt.Errorf("could not assign executor to Windows job object: %w", err)
	}

	return nil
}

// Cleanup any still hanging user processes
func (e *UniversalExecutor) killProcessTree(proc *os.Process) error {
	// We must first verify if the process is still running.
	// (Windows process often lingered around after being reported as killed).
	handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE|syscall.SYNCHRONIZE|syscall.PROCESS_QUERY_INFORMATION, false, uint32(proc.Pid))
	if err != nil {
		return os.NewSyscallError("OpenProcess", err)
	}
	defer syscall.CloseHandle(handle)

	result, err := syscall.WaitForSingleObject(syscall.Handle(handle), 0)

	switch result {
	case syscall.WAIT_OBJECT_0:
		return nil
	case syscall.WAIT_TIMEOUT:
		// Process still running.  Just kill it.
		return proc.Kill()
	default:
		return os.NewSyscallError("WaitForSingleObject", err)
	}
}

// Send a Ctrl-Break signal for shutting down the process,
func sendCtrlBreak(pid int) error {
	err := windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(pid))
	if err != nil {
		return fmt.Errorf("Error sending ctrl-break event: %v", err)
	}
	return nil
}

// Send the process a Ctrl-Break event, allowing it to shutdown by itself
// before being Terminate.
func (e *UniversalExecutor) shutdownProcess(_ os.Signal, proc *os.Process) error {
	if err := sendCtrlBreak(proc.Pid); err != nil {
		return fmt.Errorf("executor shutdown error: %v", err)
	}
	e.logger.Debug("sent Ctrl-Break to process", "pid", proc.Pid)

	return nil
}
