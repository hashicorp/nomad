// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package winappcontainer

import (
	"errors"
	"fmt"
	"regexp"
	"syscall"
	"unsafe"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/winexec"
	"golang.org/x/sys/windows"
)

var (
	userenvDLL                                    = windows.NewLazySystemDLL("userenv.dll")
	procCreateAppContainerProfile                 = userenvDLL.NewProc("CreateAppContainerProfile")
	procDeleteAppContainerProfile                 = userenvDLL.NewProc("DeleteAppContainerProfile")
	procDeriveAppContainerSidFromAppContainerName = userenvDLL.NewProc("DeriveAppContainerSidFromAppContainerName")

	ErrAccessDeniedToCreateSandbox = errors.New("Nomad does not have sufficient permission to create the template rendering AppContainer")
	ErrInvalidArg                  = errors.New("Windows returned E_INVALIDARG, this is a bug in Nomad")

	invalidContainerName = regexp.MustCompile(`[^-_. A-Za-z0-9]+`)
)

const (
	// https://learn.microsoft.com/en-us/windows/win32/fileio/file-access-rights-constants
	FILE_ALL_ACCESS uint32 = 2032127

	// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute
	PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES uint32 = 0x20009 // 131081

	// https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-createappcontainerprofile
	WindowsResultOk               uintptr = 0x0        // S_OK
	WindowsResultErrAccessDenied  uintptr = 0x80070005 // E_ACCESS_DENIED
	WindowsResultErrAlreadyExists uintptr = 0x800700b7 // HRESULT_FROM_WIN32(ERROR_ALREADY_EXISTS)
	WindowsResultErrInvalidArg    uintptr = 0x80070057 // E_INVALIDARG
	WindowsResultBadEnvironment   uintptr = 0x8007000a // BAD_ENVIRONMENT

	ExitCodeFatal int = 13 // typically this is going to be a bug in Nomad

	// sidBufferSz is the size of the buffer that the PSID will be written
	// to. The sys/x/windows.LookupSID method gets a INSUFFICIENT_BUFFER error
	// that is uses to retry with a larger size, but the methods we're calling
	// don't. Empirically, the buffer is getting populated by a *pointer* to the
	// PSID, so this should only need to be a 64-bit word long, but the failure
	// mode if we're wrong breaks template rendering, so give ourselves some
	// room to screw it up.
	sidBufferSz int = 128
)

func cleanupSID(sid *windows.SID) func() {
	return func() {
		windows.FreeSid(sid)
	}
}

func taskIDtoContainerName(id string) string {
	return trimString(invalidContainerName.ReplaceAllString(id, "-"), 64)
}

func trimString(s string, max int) string {
	if s == "" {
		// makes testing easier to handle this gracefully
		return "appcontainer"
	}
	if max > len(s) {
		max = len(s)
	}
	max = max - 1 // less a trailing NULL
	return s[:max]
}

type AppContainerConfig struct {
	Name         string
	AllowedPaths []string
}

func CreateAppContainer(log hclog.Logger, cfg *AppContainerConfig) error {
	sid, cleanup, err := createAppContainerProfile(log, cfg.Name)
	if err != nil {
		return fmt.Errorf("could not create AppContainer profile: %w", err)
	}
	defer cleanup()

	for _, path := range cfg.AllowedPaths {
		err := allowNamedObjectAccess(log, sid, path)
		if err != nil {
			return fmt.Errorf("could not grant object access: %w", err)
		}
	}

	return nil
}

func createAppContainerProfile(log hclog.Logger, taskID string) (*windows.SID, func(), error) {

	containerName := taskIDtoContainerName(taskID)
	pszAppContainerName, err := windows.UTF16PtrFromString(containerName)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"container name %q could not be encoded to utf16: %w", containerName, err)
	}

	taskID = trimString(taskID, 512)
	pszDisplayName, err := windows.UTF16PtrFromString(taskID)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"task ID %q could not be encoded to utf16: %w", taskID, err)
	}

	pszDescription, err := windows.UTF16PtrFromString(
		"template renderer AppContainer for " + taskID)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"description for task ID %q could not be encoded to utf16: %w", taskID, err)
	}

	var pCapabilities uintptr    // PSID_AND_ATTRIBUTES
	var dwCapabilityCount uint32 // DWORD

	// note: this buffer gets populated with a pointer to a PSID, and the
	// resulting handle needs to be freed here in the caller
	sidBuffer := make([]byte, sidBufferSz)

	// 	USERENVAPI HRESULT CreateAppContainerProfile(
	//   [in]  PCWSTR              pszAppContainerName,
	//   [in]  PCWSTR              pszDisplayName,
	//   [in]  PCWSTR              pszDescription,
	//   [in]  PSID_AND_ATTRIBUTES pCapabilities,
	//   [in]  DWORD               dwCapabilityCount,
	//   [out] PSID                *ppSidAppContainerSid
	// );
	// https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-createappcontainerprofile
	result, _, err := procCreateAppContainerProfile.Call(
		uintptr(unsafe.Pointer(pszAppContainerName)),
		uintptr(unsafe.Pointer(pszDisplayName)),
		uintptr(unsafe.Pointer(pszDescription)),
		uintptr(pCapabilities),
		uintptr(dwCapabilityCount),
		uintptr(unsafe.Pointer(&sidBuffer)),
	)
	ppSidAppContainerSid := (*windows.SID)(unsafe.Pointer(&sidBuffer[0]))

	switch result {
	case WindowsResultOk:
		if !ppSidAppContainerSid.IsValid() {
			return nil, nil, fmt.Errorf("creating AppContainer returned invalid SID: %v",
				ppSidAppContainerSid.String())
		}

		log.Debug("created new AppContainer", "sid", ppSidAppContainerSid.String())
		return ppSidAppContainerSid, cleanupSID(ppSidAppContainerSid), nil

	case WindowsResultErrAccessDenied, WindowsResultBadEnvironment:
		// we cannot sandbox if Nomad is running with insufficient privs, so in
		// that case we rely on the file system permissions that the user gave
		// Nomad
		return nil, nil, ErrAccessDeniedToCreateSandbox

	case WindowsResultErrAlreadyExists:
		// WARNING: this method will return a derived SID even if the container
		// doesn't already exist, so it's critical that we don't "optimize" this
		// method by checking first!
		return deriveAppContainerSID(taskID)

	case WindowsResultErrInvalidArg:
		return nil, nil, ErrInvalidArg

	default:
		// note: the error we get here is always non-nil and always reports
		// sucess for known error codes
		return nil, nil, fmt.Errorf("creating AppContainer failed: (%x) %v",
			result, syscall.Errno(result))
	}

}

// deriveAppContainerSID gets the AppContainer SID that should be associated
// with the given task ID. Note that if the AppContainer exists, Windows will
// give us the SID that it should have, so we can only call this if we know that
// we've already created the AppContainer
func deriveAppContainerSID(taskID string) (*windows.SID, func(), error) {

	containerName := taskIDtoContainerName(taskID)
	pszAppContainerName, err := windows.UTF16PtrFromString(containerName)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"container name %q could not be encoded to utf16: %w", containerName, err)
	}

	// note: this buffer gets populated with a pointer to a PSID, and the
	// resulting handle needs to be freed here in the caller
	sidBuffer := make([]byte, sidBufferSz)

	// USERENVAPI HRESULT DeriveAppContainerSidFromAppContainerName(
	//   [in]  PCWSTR pszAppContainerName,
	//   [out] PSID   *ppsidAppContainerSid
	// );
	// https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-deriveappcontainersidfromappcontainername
	result, _, err := procDeriveAppContainerSidFromAppContainerName.Call(
		uintptr(unsafe.Pointer(pszAppContainerName)),
		uintptr(unsafe.Pointer(&sidBuffer)),
	)
	switch result {
	case WindowsResultOk:
		ppSidAppContainerSid := (*windows.SID)(unsafe.Pointer(&sidBuffer[0]))
		if !ppSidAppContainerSid.IsValid() {
			return nil, nil, fmt.Errorf("looking up AppContainer SID returned invalid SID: %v",
				ppSidAppContainerSid.String())
		}

		return ppSidAppContainerSid, cleanupSID(ppSidAppContainerSid), nil
	default:
		return nil, nil, fmt.Errorf("looking up AppContainer SID failed: errno=%v, err=%w",
			syscall.Errno(result), err)
	}
}

// allowNamedObjectAccess grants inheritable R/W access to the object path for
// the AppContainer SID
func allowNamedObjectAccess(log hclog.Logger, sid *windows.SID, path string) error {
	pathAccess := windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.ACCESS_MASK(FILE_ALL_ACCESS),
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE,
		Trustee: windows.TRUSTEE{
			MultipleTrustee:          nil,
			MultipleTrusteeOperation: windows.NO_MULTIPLE_TRUSTEE,
			TrusteeForm:              windows.TRUSTEE_IS_SID,
			TrusteeType:              windows.TRUSTEE_IS_GROUP,
			TrusteeValue:             windows.TrusteeValueFromSID(sid),
		},
	}

	pathSD, err := windows.GetNamedSecurityInfo(
		path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("could not GetNamedSecurityInfo for %q: %w", path, err)
	}

	acl, _, err := pathSD.DACL()
	if err != nil {
		return fmt.Errorf("could not get DACL for %q: %w", path, err)
	}

	newACL, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{pathAccess}, acl)
	if err != nil {
		return fmt.Errorf("could not create new DACL for %q: %w", path, err)
	}

	err = windows.SetNamedSecurityInfo(
		path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, newACL, nil)
	if err != nil {
		return fmt.Errorf("could not SetNamedSecurityInfo for %q: %w", path, err)
	}

	log.Trace("AppContainer access configured", "sid", sid, "path", path)
	return nil
}

func DeleteAppContainer(log hclog.Logger, taskID string) error {

	containerName := taskIDtoContainerName(taskID)
	pszAppContainerName, err := windows.UTF16PtrFromString(containerName)
	if err != nil {
		return fmt.Errorf(
			"container name %q could not be encoded to utf16: %w", containerName, err)
	}

	// USERENVAPI HRESULT DeleteAppContainerProfile(
	//   [in] PCWSTR pszAppContainerName
	// );
	// https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-deleteappcontainerprofile
	result, _, err := procDeleteAppContainerProfile.Call(
		uintptr(unsafe.Pointer(pszAppContainerName)),
	)

	switch result {
	case WindowsResultOk: // we get this if AppContainer doesn't exist
		log.Debug("deleted AppContainer")
		return nil

	case WindowsResultErrInvalidArg:
		return ErrInvalidArg

	default:
		// note: the error we get here is always non-nil and always reports
		// sucess for known error codes
		return fmt.Errorf("deleting AppContainer failed: errno=%v, err=%w",
			syscall.Errno(result), err)
	}

}

func CreateProcThreadAttributes(taskID string) ([]winexec.ProcThreadAttribute, func(), error) {

	sid, cleanup, err := deriveAppContainerSID(taskID)
	if err != nil {
		return nil, cleanup, fmt.Errorf("could not get SID for app container: %w", err)
	}

	procThreadAttrs, err := createProcThreadAttributes(sid)
	if err != nil {
		return nil, cleanup, fmt.Errorf("could not create proc attributes: %w", err)
	}

	return procThreadAttrs, cleanup, nil
}

type SecurityCapabilities struct {
	AppContainerSid uintptr // PSID *windows.SID
	Capabilities    uintptr // SID_AND_ATTRIBUTES *windows.SIDAndAttributes
	CapabilityCount uint32
	Reserved        uint32
}

// createProcThreadAttributes returns ProcThreadAttributes so that winexec.Cmd
// can set the SecurityCapabilities on the process
func createProcThreadAttributes(containerSID *windows.SID) ([]winexec.ProcThreadAttribute, error) {

	sd, err := windows.NewSecurityDescriptor()
	if err != nil {
		return nil, fmt.Errorf("could not create new security descriptor: %w", err)
	}
	sd.SetOwner(containerSID, true)

	sc := &SecurityCapabilities{AppContainerSid: uintptr(unsafe.Pointer(containerSID))}

	return []winexec.ProcThreadAttribute{
		{
			Attribute: uintptr(PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES),
			Value:     unsafe.Pointer(sc),
			Size:      uintptr(unsafe.Sizeof(*sc)),
		}}, nil
}
