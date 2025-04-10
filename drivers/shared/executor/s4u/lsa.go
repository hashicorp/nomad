// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package s4u

import (
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modsecur32 = windows.NewLazySystemDLL("secur32.dll")

	procLsaRegisterLogonProcess        = modsecur32.NewProc("LsaRegisterLogonProcess")
	procLsaDeregisterLogonProcess      = modsecur32.NewProc("LsaDeregisterLogonProcess")
	procLsaLookupAuthenticationPackage = modsecur32.NewProc("LsaLookupAuthenticationPackage")
	procLsaLogonUser                   = modsecur32.NewProc("LsaLogonUser")
	procLsaFreeReturnBuffer            = modsecur32.NewProc("LsaFreeReturnBuffer")
)

type LSA_STRING struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *byte
}

type LSA_UNICODE_STRING struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

func mustStringToLsaString(s string) *LSA_STRING {
	var result LSA_STRING
	var err error
	result.Length = uint16(len(s))
	result.MaximumLength = uint16(len(s))
	result.Buffer, err = windows.BytePtrFromString(s)
	if err != nil {
		log.Fatal(err)
	}
	return &result
}

func LsaRegisterLogonProcess(logonProcessName *LSA_STRING, handle *windows.Handle) error {
	var mode uint32
	status, _, _ := syscall.SyscallN(
		procLsaRegisterLogonProcess.Addr(),
		uintptr(unsafe.Pointer(logonProcessName)),
		uintptr(unsafe.Pointer(handle)),
		uintptr(unsafe.Pointer(&mode)))

	if status != 0 {
		return windows.NTStatus(status)
	}
	return nil
}

func LsaDeregisterLogonProcess(hnd windows.Handle) error {
	status, _, _ := syscall.SyscallN(procLsaDeregisterLogonProcess.Addr(), uintptr(hnd))
	if status != 0 {
		return windows.NTStatus(status)
	}
	return nil
}

func LsaLookupAuthenticationPackage(hnd windows.Handle, packageName *LSA_STRING, authPackageId *uint32) error {
	status, _, _ := syscall.SyscallN(
		procLsaLookupAuthenticationPackage.Addr(),
		uintptr(hnd),
		uintptr(unsafe.Pointer(packageName)),
		uintptr(unsafe.Pointer(authPackageId)))

	if status != 0 {
		return windows.NTStatus(status)
	}
	return nil
}

type SecurityLogonType uint32

const (
	LogonTypeUndefined   = 0
	LogonTypeInteractive = 1 + iota
	LogonTypeNetwork
	LogonTypeBatch
	LogonTypeService
	LogonTypeProxy
	LogonTypeUnlock
	LogonTypeNetworkCleartext
	LogonTypeNewCredentials
	LogonTypeRemoteInteractive
	LogonTypeCachedInteractive
	LogonTypeCachedRemoteInteractive
	LogonTypeCachedUnlock
)

type TokenGroups struct {
	PrivilegeCount uint32
	Privileges     [1]windows.LUIDAndAttributes
}
type TokenSource struct {
	SourceName [8]byte
	SourceId   windows.LUID
}
type QuotaLimits struct {
	PagedPoolLimit        uintptr
	NonPagedPoolLimit     uintptr
	MinimumWorkingSetSize uintptr
	MaximumWorkingSetSize uintptr
	PagefileLimit         uintptr
	TimeLimit             uint64
}

func LsaLogonUser(hnd windows.Handle,
	originName *LSA_STRING,
	logonType SecurityLogonType,
	authPackageId uint32,
	accountInformation *byte, accountInformationLength uint32,
	tokenGroups *TokenGroups,
	tokenSource *TokenSource,
	profileBuffer **byte, profileBufferLength *uint32,
	logonId *windows.LUID,
	token *windows.Token,
	quotas *QuotaLimits,
	subStatus *windows.NTStatus) error {
	status, _, _ := syscall.SyscallN(
		procLsaLogonUser.Addr(),
		uintptr(hnd),
		uintptr(unsafe.Pointer(originName)),
		uintptr(logonType),
		uintptr(authPackageId),
		uintptr(unsafe.Pointer(accountInformation)),
		uintptr(accountInformationLength),
		uintptr(unsafe.Pointer(tokenGroups)),
		uintptr(unsafe.Pointer(tokenSource)),
		uintptr(unsafe.Pointer(profileBuffer)),
		uintptr(unsafe.Pointer(profileBufferLength)),
		uintptr(unsafe.Pointer(logonId)),
		uintptr(unsafe.Pointer(token)),
		uintptr(unsafe.Pointer(quotas)),
		uintptr(unsafe.Pointer(subStatus)))

	if status != 0 {
		return windows.NTStatus(status)
	}
	return nil
}

type MSV1_0_S4U_LOGON struct {
	MessageType       MSV1_0_LOGON_SUBMIT_TYPE
	Flags             uint32
	UserPrincipalName LSA_UNICODE_STRING // username or username@domain
	DomainName        LSA_UNICODE_STRING // Optional: if missing, using the local machine
}

type MSV1_0_LOGON_SUBMIT_TYPE uint32

const (
	MsV1_0InteractiveLogon       = 2
	MsV1_0Lm20Logon              = 3
	MsV1_0NetworkLogon           = 4
	MsV1_0SubAuthLogon           = 5
	MsV1_0WorkstationUnlockLogon = 7
	MsV1_0S4ULogon               = 12
	MsV1_0VirtualLogon           = 82
)

type KERB_S4U_LOGON struct {
	MessageType KERB_LOGON_SUBMIT_TYPE
	Flags       uint32
	ClientUpn   LSA_UNICODE_STRING
	ClientRealm LSA_UNICODE_STRING
}

type KERB_LOGON_SUBMIT_TYPE uint32

const (
	KerbInteractiveLogon       = 2
	KerbSmartCardLogon         = 6
	KerbWorkstationUnlockLogon = 7
	KerbSmartCardUnlockLogon   = 8
	KerbProxyLogon             = 9
	KerbTicketLogon            = 10
	KerbTicketUnlockLogon      = 11
	KerbS4ULogon               = 12
	KerbCertificateLogon       = 13
	KerbCertificateS4ULogon    = 14
	KerbCertificateUnlockLogon = 15
)

func LsaFreeReturnBuffer(buff *byte) error {
	status, _, _ := syscall.SyscallN(procLsaFreeReturnBuffer.Addr(), uintptr(unsafe.Pointer(buff)))
	if status != 0 {
		return windows.NTStatus(status)
	}
	return nil
}
