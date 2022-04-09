package capabilities

import (
	"fmt"
	"regexp"

	"github.com/syndtr/gocapability/capability"
)

const (
	// HCLSpecLiteral is an equivalent list to NomadDefaults, expressed as a literal
	// HCL string for use in HCL config parsing.
	HCLSpecLiteral = `["AUDIT_WRITE","CHOWN","DAC_OVERRIDE","FOWNER","FSETID","KILL","MKNOD","NET_BIND_SERVICE","SETFCAP","SETGID","SETPCAP","SETUID","SYS_CHROOT"]`
)

var (
	extractLiteral = regexp.MustCompile(`([\w]+)`)
)

// NomadDefaults is the set of Linux capabilities that Nomad enables by
// default. This list originates from what Docker enabled by default, but then
// excludes NET_RAW for security reasons.
//
// This set is use in the as HCL configuration default, described by HCLSpecLiteral.
func NomadDefaults() *Set {
	return New(extractLiteral.FindAllString(HCLSpecLiteral, -1))
}

// DockerDefaults is a list of Linux capabilities enabled by Docker by default
// and is used to compute the set of capabilities to add/drop given docker driver
// configuration.
//
// https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities
func DockerDefaults() *Set {
	defaults := NomadDefaults()
	defaults.Add("NET_RAW")
	return defaults
}

// Supported returns the set of capabilities supported by the operating system.
//
// This set will expand over time as new capabilities are introduced to the kernel
// and the capability library is updated (which tends to happen to keep up with
// run-container libraries).
//
// Defers to a library generated from
// https://github.com/torvalds/linux/blob/master/include/uapi/linux/capability.h
func Supported() *Set {
	s := New(nil)

	last := capability.CAP_LAST_CAP

	// workaround for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}

	// accumulate every capability supported by this system
	for _, c := range capability.List() {
		if c > last {
			continue
		}
		s.Add(c.String())
	}

	return s
}

// LegacySupported returns the historical set of capabilities used when a task is
// configured to run as root using the exec task driver. Older versions of Nomad
// always allowed the root user to make use of any capability. Now that the exec
// task driver supports configuring the allowed capabilities, operators are
// encouraged to explicitly opt-in to capabilities beyond this legacy set. We
// maintain the legacy list here, because previous versions of Nomad deferred to
// the capability.List library function, which adds new capabilities over time.
//
// https://github.com/hashicorp/nomad/blob/v1.0.4/vendor/github.com/syndtr/gocapability/capability/enum_gen.go#L88
func LegacySupported() *Set {
	return New([]string{
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_DAC_READ_SEARCH",
		"CAP_FOWNER",
		"CAP_FSETID",
		"CAP_KILL",
		"CAP_SETGID",
		"CAP_SETUID",
		"CAP_SETPCAP",
		"CAP_LINUX_IMMUTABLE",
		"CAP_NET_BIND_SERVICE",
		"CAP_NET_BROADCAST",
		"CAP_NET_ADMIN",
		"CAP_NET_RAW",
		"CAP_IPC_LOCK",
		"CAP_IPC_OWNER",
		"CAP_SYS_MODULE",
		"CAP_SYS_RAWIO",
		"CAP_SYS_CHROOT",
		"CAP_SYS_PTRACE",
		"CAP_SYS_PACCT",
		"CAP_SYS_ADMIN",
		"CAP_SYS_BOOT",
		"CAP_SYS_NICE",
		"CAP_SYS_RESOURCE",
		"CAP_SYS_TIME",
		"CAP_SYS_TTY_CONFIG",
		"CAP_MKNOD",
		"CAP_LEASE",
		"CAP_AUDIT_WRITE",
		"CAP_AUDIT_CONTROL",
		"CAP_SETFCAP",
		"CAP_MAC_OVERRIDE",
		"CAP_MAC_ADMIN",
		"CAP_SYSLOG",
		"CAP_WAKE_ALARM",
		"CAP_BLOCK_SUSPEND",
		"CAP_AUDIT_READ",
	})
}

// Calculate the resulting set of linux capabilities to enable for a task, taking
// into account:
// - default capability basis
// - driver allowable capabilities
// - task capability drops
// - task capability adds
//
// Nomad establishes a standard set of enabled capabilities allowed by the task
// driver if allow_caps is not set. This is the same set that the task will be
// enabled with by default if allow_caps does not further reduce permissions,
// in which case the task capabilities will also be reduced accordingly.
//
// The task will drop any capabilities specified in cap_drop, and add back
// capabilities specified in cap_add. The task will not be allowed to add capabilities
// not set in the the allow_caps setting (which by default is the same as the basis).
//
// cap_add takes precedence over cap_drop, enabling the common pattern of dropping
// all capabilities, then adding back the desired smaller set. e.g.
//   cap_drop = ["all"]
//   cap_add = ["chown", "kill"]
//
// Note that the resulting capability names are upper-cased and prefixed with
// "CAP_", which is the expected input for the exec/java driver implementation.
func Calculate(basis *Set, allowCaps, capAdd, capDrop []string) ([]string, error) {
	allow := New(allowCaps)
	adds := New(capAdd)

	// determine caps the task wants that are not allowed
	missing := allow.Difference(adds)
	if !missing.Empty() {
		return nil, fmt.Errorf("driver does not allow the following capabilities: %s", missing)
	}

	// the realized enabled capabilities starts with what is allowed both by driver
	// config AND is a member of the basis (i.e. nomad defaults)
	result := basis.Intersect(allow)

	// then remove capabilities the task explicitly drops
	result.Remove(capDrop)

	// then add back capabilities the task explicitly adds
	return result.Union(adds).Slice(true), nil
}

// Delta calculates the set of capabilities that must be added and dropped relative
// to a basis to achieve a desired result. The use case is that the docker driver
// assumes a default set (DockerDefault), and we must calculate what to pass into
// --cap-add and --cap-drop on container creation given the inputs of the docker
// plugin config for allow_caps, and the docker task configuration for cap_add and
// cap_drop. Note that the user provided cap_add and cap_drop settings are always
// included, even if they are redundant with the basis (maintaining existing
// behavior, working with existing tests).
//
// Note that the resulting capability names are lower-cased and not prefixed with
// "CAP_", which is the existing style used with the docker driver implementation.
func Delta(basis *Set, allowCaps, capAdd, capDrop []string) ([]string, []string, error) {
	all := func(caps []string) bool {
		for _, c := range caps {
			if normalize(c) == "all" {
				return true
			}
		}
		return false
	}

	// set of caps allowed by driver
	allow := New(allowCaps)

	// determine caps the task wants that are not allowed
	missing := allow.Difference(New(capAdd))
	if !missing.Empty() {
		return nil, nil, fmt.Errorf("driver does not allow the following capabilities: %s", missing)
	}

	// add what the task is asking for
	add := New(capAdd).Slice(false)
	if all(capAdd) {
		add = []string{"all"}
	}

	// drop what the task removes plus whatever is in the basis that is not
	// in the driver allow configuration
	drop := New(allowCaps).Difference(basis).Union(New(capDrop)).Slice(false)
	if all(capDrop) {
		drop = []string{"all"}
	}

	return add, drop, nil
}
