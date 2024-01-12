// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package winexec

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var EINVAL = errors.New("EINVAL")

func (c *Cmd) createProcess(
	path string, commandLine []string,
	userProcThreadAttrs []ProcThreadAttribute,
	attr *syscall.ProcAttr,
) (*os.Process, error) {

	// Much like in os/exec Command, we're creating the process directly without
	// creating a shell. Unlike what we're doing for Linux/Unix, we're creating
	// this process directly into the AppContainer rather than starting the
	// process and dropping privs, and we control all the initial arguments that
	// enforce we're calling the particular binary we want.
	cli := windows.ComposeCommandLine(commandLine)
	wCommandLine, err := windows.UTF16PtrFromString(cli)
	if err != nil {
		return nil, fmt.Errorf("could not create UTF16 pointer from cli: %w", err)
	}

	var wCurrentDir *uint16
	if c.Dir != "" {
		wCurrentDir, err = windows.UTF16PtrFromString(c.Dir)
		if err != nil {
			return nil, fmt.Errorf("could not create UTF16 pointer from currentDir: %w", err)
		}
	}

	parentProcess, _ := windows.GetCurrentProcess()
	p := parentProcess
	fd := make([]windows.Handle, len(attr.Files))
	for i := range attr.Files {
		if attr.Files[i] > 0 {
			destinationProcessHandle := parentProcess
			err := windows.DuplicateHandle(
				p, windows.Handle(attr.Files[i]),
				destinationProcessHandle, &fd[i], 0, true, windows.DUPLICATE_SAME_ACCESS)
			if err != nil {
				return nil, err
			}
			defer windows.DuplicateHandle(
				parentProcess, fd[i], 0, nil, 0, false, windows.DUPLICATE_CLOSE_SOURCE)
		}
	}

	procThreadAttrs, err := mergeProcThreadAttrs(fd, userProcThreadAttrs)
	if err != nil {
		return nil, err
	}

	startupInfo := new(windows.StartupInfoEx)
	startupInfo.Cb = uint32(unsafe.Sizeof(*startupInfo)) // Cb: size of struct in bytes
	startupInfo.ProcThreadAttributeList = procThreadAttrs.List()
	startupInfo.StdInput = fd[0]
	startupInfo.StdOutput = fd[1]
	startupInfo.StdErr = fd[2]
	startupInfo.Flags = syscall.STARTF_USESTDHANDLES

	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT |
		windows.EXTENDED_STARTUPINFO_PRESENT)

	envBlock, err := createEnvBlock(attr.Env)
	if err != nil {
		return nil, err
	}

	outProcInfo := new(windows.ProcessInformation)
	err = windows.CreateProcess(
		nil, //appName
		wCommandLine,
		nil,  // procSecurity
		nil,  // threadSecurity
		true, // inheritHandles,
		flags,
		envBlock,
		wCurrentDir,
		&startupInfo.StartupInfo,
		outProcInfo)
	if err != nil {
		return nil, fmt.Errorf("could not CreateProcess: %w", err)
	}

	defer windows.CloseHandle(windows.Handle(outProcInfo.Thread))

	// this ensures we don't call the finalizers on the attr.Files before we
	// make the syscall. See stdlib's os/exec_posix.go for another example.
	runtime.KeepAlive(fd)
	runtime.KeepAlive(attr)

	return os.FindProcess(int(outProcInfo.ProcessId))
}

// ref https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute
// actual value from https://docs.rs/windows-sys/latest/windows_sys/Win32/System/Threading/constant.PROC_THREAD_ATTRIBUTE_HANDLE_LIST.html and empirically tested
const PROC_THREAD_ATTRIBUTE_HANDLE_LIST = 0x20002 // 131074

type ProcThreadAttribute struct {
	Attribute uintptr
	Value     unsafe.Pointer
	Size      uintptr
}

func mergeProcThreadAttrs(
	fd []windows.Handle,
	userAttrs []ProcThreadAttribute,
) (*windows.ProcThreadAttributeListContainer, error) {

	newLen := len(userAttrs) + 1

	procThreadAttrs, err := windows.NewProcThreadAttributeList(uint32(newLen))
	if err != nil {
		return nil, fmt.Errorf("could not create NewProcThreadAttributeList: %v", err)
	}

	err = procThreadAttrs.Update(
		uintptr(PROC_THREAD_ATTRIBUTE_HANDLE_LIST),
		unsafe.Pointer(&fd[0]),
		uintptr(len(fd))*unsafe.Sizeof(fd[0]))
	if err != nil {
		return nil, fmt.Errorf("could not update procthread attrs: %v", err)
	}

	for _, userAttr := range userAttrs {
		err = procThreadAttrs.Update(
			userAttr.Attribute,
			userAttr.Value,
			userAttr.Size)
		if err != nil {
			return nil, fmt.Errorf("could not update procthread attrs: %v", err)
		}
	}

	return procThreadAttrs, nil
}
