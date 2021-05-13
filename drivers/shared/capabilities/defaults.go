package capabilities

import (
	"regexp"

	"github.com/syndtr/gocapability/capability"
)

const (
	// HCLSpecLiteral is an equivalent list to NomadDefaults, expressed as a literal
	// HCL string for use in HCL config parsing.
	HCLSpecLiteral = `["AUDIT_WRITE","CHOWN","DAC_OVERRIDE","FOWNER","FSETID","KILL","MKNOD","NET_BIND_SERVICE","SETFCAP","SETGID","SETPCAP","SETUID","SYS_CHROOT"]`
)

var (
	extractLiteral = regexp.MustCompile(`("[\w]+)`)
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
