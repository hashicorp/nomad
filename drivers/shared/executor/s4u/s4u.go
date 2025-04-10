//go:build windows
// +build windows

package s4u

import (
	"fmt"
	"math"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Best intro to Windows S4U is probably here:
// https://learn.microsoft.com/en-us/archive/msdn-magazine/2003/april/exploring-s4u-kerberos-extensions-in-windows-server-2003

const MSV1_0_PACKAGE_NAME = "MICROSOFT_AUTHENTICATION_PACKAGE_V1_0"
const MICROSOFT_KERBEROS_NAME = "Kerberos"

// From the LsaLogonUser documentation:
// A token source identifies the source module — for example,
// the session manager—and the context that may be useful to that module.
// This information is included in the user token and can be retrieved by
// calling GetTokenInformation.
const TOKEN_SOURCE_NAME = "nomad_raw_exec"

// From the LsaRegisterLogonProcess documentation:
// Name identifying the logon application.
// This should be a printable name suitable for display to administrators.
// For example, the Windows logon application might use the name "User32LogonProcess".
// This name is used by the LSA during auditing.
const LOGON_PROCESS_NAME = "nomad_raw_exec"

// A string that identifies the origin of the logon attempt
// From the LsaLogonUser documentation:
//
//	The OriginName parameter should specify meaningful information.
//	For example, it might contain "TTY1" to indicate terminal one or
//	"NTLM - remote node JAZZ" to indicate a network logon that uses
//	NTLM through a remote node called "JAZZ".
//
// Not overly helpful.
const LOGON_ORIGIN_NAME = "nomad_raw_exec"

func createTokenSource(sourceName string) (TokenSource, error) {
	var result TokenSource
	if err := AllocateLocallyUniqueId(&result.SourceId); err != nil {
		return result, err
	}

	for i, c := range []byte(sourceName) {
		if i == 8 {
			break
		}
		result.SourceName[i] = c
	}
	return result, nil
}

func copyUtf16ToBytes(s []uint16, stringbuf []byte) {
	for i, c := range s {
		*(*uint16)(unsafe.Pointer(&stringbuf[i*2])) = c
	}
}

func buildContiguousLsaUnicodeString(result *LSA_UNICODE_STRING, s []uint16, stringbuf []byte) error {
	bytelen := len(s) * 2
	if bytelen > math.MaxUint16 {
		return fmt.Errorf("String too long to for API : %d", bytelen)
	}
	if bytelen > len(stringbuf) {
		return fmt.Errorf("Insufficient buffer space, require %d, got %d", bytelen, len(stringbuf))
	}

	copyUtf16ToBytes(s, stringbuf)

	result.Length = uint16(bytelen)
	result.MaximumLength = uint16(bytelen)
	result.Buffer = (*uint16)(unsafe.Pointer(&stringbuf[0]))
	return nil
}

func sizeofMSV1_0_S4U_LOGON() uintptr {
	var temp MSV1_0_S4U_LOGON
	return unsafe.Sizeof(temp)
}

func buildLocalS4uLogonInfo(username string) ([]byte, uint32, error) {
	usernameUtf16 := utf16.Encode([]rune(username))
	usernameUtf16Bytelen := len(usernameUtf16) * 2
	domainUtf16 := utf16.Encode([]rune("."))
	domainUf16Bytelen := len(usernameUtf16) * 2

	accountInformationLength := sizeofMSV1_0_S4U_LOGON() + uintptr(usernameUtf16Bytelen+domainUf16Bytelen)
	accountInformation := make([]byte, accountInformationLength)

	var offset uintptr = 0
	s4uLogon := (*MSV1_0_S4U_LOGON)(unsafe.Pointer(&accountInformation[offset]))
	s4uLogon.MessageType = MsV1_0S4ULogon
	offset += unsafe.Sizeof(*s4uLogon)

	err := buildContiguousLsaUnicodeString(&s4uLogon.UserPrincipalName, usernameUtf16, accountInformation[offset:])
	if err != nil {
		return nil, 0, fmt.Errorf("Error building UserPrincipalName buffer : %w", err)
	}
	offset += uintptr(usernameUtf16Bytelen)

	err = buildContiguousLsaUnicodeString(&s4uLogon.DomainName, domainUtf16, accountInformation[offset:])
	if err != nil {
		return nil, 0, fmt.Errorf("Error building DomainName buffer : %w", err)
	}
	offset += uintptr(domainUf16Bytelen)

	return accountInformation, uint32(offset), nil
}

func sizeofKERB_S4U_LOGON() uintptr {
	var temp KERB_S4U_LOGON
	return unsafe.Sizeof(temp)
}

func buildDomainS4uLogonInfo(userUpn string) ([]byte, uint32, error) {
	upnUtf16 := utf16.Encode([]rune(userUpn))
	upnUtf16ByteLen := len(upnUtf16) * 2

	accountInformationLength := sizeofKERB_S4U_LOGON() + uintptr(len(userUpn)*2)
	accountInformation := make([]byte, accountInformationLength)

	var offset uintptr = 0
	s4uLogon := (*KERB_S4U_LOGON)(unsafe.Pointer(&accountInformation[offset]))
	s4uLogon.MessageType = MsV1_0S4ULogon
	offset += unsafe.Sizeof(*s4uLogon)

	s4uLogon.ClientUpn.Length = uint16(upnUtf16ByteLen)
	s4uLogon.ClientUpn.MaximumLength = uint16(upnUtf16ByteLen)
	s4uLogon.ClientUpn.Buffer = (*uint16)(unsafe.Pointer(&accountInformation[offset]))

	copyUtf16ToBytes(upnUtf16, accountInformation[offset:])
	offset += uintptr(upnUtf16ByteLen)

	return accountInformation, uint32(offset), nil
}

func lsaLogonUser(logonProcessHnd windows.Handle, authPackageId uint32, accountInformation []byte, accountInformationLength uint32) (result windows.Token, err error) {
	var profileLen uint32
	var profileBuffer *byte
	var logonId windows.LUID
	var quotas QuotaLimits
	var substatus windows.NTStatus
	var tokenSource TokenSource

	if tokenSource, err = createTokenSource(TOKEN_SOURCE_NAME); err != nil {
		return result, fmt.Errorf("Error creating token source : %w", err)
	}

	err = LsaLogonUser(logonProcessHnd,
		mustStringToLsaString("nomad"),
		LogonTypeNetwork,
		authPackageId,
		&accountInformation[0],
		accountInformationLength,
		nil,
		&tokenSource,
		&profileBuffer,
		&profileLen,
		&logonId,
		&result,
		&quotas,
		&substatus)
	if err != nil {
		return result, fmt.Errorf("Error calling LsaLogonUser : %w, substatus : %v", err, substatus)
	}

	_ = LsaFreeReturnBuffer(profileBuffer)

	return result, nil
}

func GetLocalS4uToken(username string) (result windows.Token, err error) {
	var logonProcessHnd windows.Handle
	if err = LsaRegisterLogonProcess(mustStringToLsaString(LOGON_PROCESS_NAME), &logonProcessHnd); err != nil {
		return result, fmt.Errorf("Error from LsaRegisterLogonProcess : %w", err)
	}

	defer func() {
		_ = LsaDeregisterLogonProcess(logonProcessHnd)
	}()

	var authPackageId uint32
	if err := LsaLookupAuthenticationPackage(logonProcessHnd, mustStringToLsaString(MSV1_0_PACKAGE_NAME), &authPackageId); err != nil {
		return result, fmt.Errorf("Error from LsaLookupAuthenticationPackage : %w", err)
	}

	accountInformation, accountInformationLength, err := buildLocalS4uLogonInfo(username)
	if err != nil {
		return result, fmt.Errorf("Error building account information buffer : %w", err)
	}

	return lsaLogonUser(logonProcessHnd, authPackageId, accountInformation, accountInformationLength)
}

func GetDomainS4uToken(upn string) (result windows.Token, err error) {
	var logonProcessHnd windows.Handle
	if err = LsaRegisterLogonProcess(mustStringToLsaString(LOGON_PROCESS_NAME), &logonProcessHnd); err != nil {
		return result, fmt.Errorf("Error from LsaRegisterLogonProcess : %w", err)
	}

	defer func() {
		_ = LsaDeregisterLogonProcess(logonProcessHnd)
	}()

	var authPackageId uint32
	if err := LsaLookupAuthenticationPackage(logonProcessHnd, mustStringToLsaString(MICROSOFT_KERBEROS_NAME), &authPackageId); err != nil {
		return result, fmt.Errorf("Error from LsaLookupAuthenticationPackage : %w", err)
	}

	accountInformation, accountInformationLength, err := buildDomainS4uLogonInfo(upn)
	if err != nil {
		return result, fmt.Errorf("Error building account information buffer : %w", err)
	}

	return lsaLogonUser(logonProcessHnd, authPackageId, accountInformation, accountInformationLength)
}
