// Copyright © 2013, 2014, The Go-LXC Authors. All rights reserved.
// Use of this source code is governed by a LGPLv2.1
// license that can be found in the LICENSE file.

// +build linux,cgo

package lxc

// #include <lxc/lxccontainer.h>
import "C"

import "fmt"

// Verbosity type
type Verbosity int

const (
	// Quiet makes some API calls not to write anything to stdout
	Quiet Verbosity = 1 << iota
	// Verbose makes some API calls write to stdout
	Verbose
)

// BackendStore type specifies possible backend types.
type BackendStore int

const (
	// Btrfs backendstore type
	Btrfs BackendStore = iota + 1
	// Directory backendstore type
	Directory
	// LVM backendstore type
	LVM
	// ZFS backendstore type
	ZFS
	// Aufs backendstore type
	Aufs
	// Overlayfs backendstore type
	Overlayfs
	// Loopback backendstore type
	Loopback
	// Best backendstore type
	Best
)

// BackendStore as string
func (t BackendStore) String() string {
	switch t {
	case Directory:
		return "dir"
	case ZFS:
		return "zfs"
	case Btrfs:
		return "btrfs"
	case LVM:
		return "lvm"
	case Aufs:
		return "aufs"
	case Overlayfs:
		return "overlayfs"
	case Loopback:
		return "loopback"
	case Best:
		return "best"
	}
	return ""
}

var backendStoreMap = map[string]BackendStore{
	"dir":       Directory,
	"zfs":       ZFS,
	"btrfs":     Btrfs,
	"lvm":       LVM,
	"aufs":      Aufs,
	"overlayfs": Overlayfs,
	"loopback":  Loopback,
	"best":      Best,
}

// Set is the method to set the flag value, part of the flag.Value interface.
func (t *BackendStore) Set(value string) error {
	backend, ok := backendStoreMap[value]
	if ok {
		*t = backend
		return nil
	}
	return ErrUnknownBackendStore
}

// State type specifies possible container states.
type State int

const (
	// STOPPED means container is not running
	STOPPED State = iota + 1
	// STARTING means container is starting
	STARTING
	// RUNNING means container is running
	RUNNING
	// STOPPING means container is stopping
	STOPPING
	// ABORTING means container is aborting
	ABORTING
	// FREEZING means container is freezing
	FREEZING
	// FROZEN means containe is frozen
	FROZEN
	// THAWED means container is thawed
	THAWED
)

// StateMap provides the mapping betweens the state names and states
var StateMap = map[string]State{
	"STOPPED":  STOPPED,
	"STARTING": STARTING,
	"RUNNING":  RUNNING,
	"STOPPING": STOPPING,
	"ABORTING": ABORTING,
	"FREEZING": FREEZING,
	"FROZEN":   FROZEN,
	"THAWED":   THAWED,
}

// State as string
func (t State) String() string {
	switch t {
	case STOPPED:
		return "STOPPED"
	case STARTING:
		return "STARTING"
	case RUNNING:
		return "RUNNING"
	case STOPPING:
		return "STOPPING"
	case ABORTING:
		return "ABORTING"
	case FREEZING:
		return "FREEZING"
	case FROZEN:
		return "FROZEN"
	case THAWED:
		return "THAWED"
	}
	return ""
}

// Taken from http://golang.org/doc/effective_go.html#constants

// ByteSize type
type ByteSize float64

const (
	_ = iota
	// KB - kilobyte
	KB ByteSize = 1 << (10 * iota)
	// MB - megabyte
	MB
	// GB - gigabyte
	GB
	// TB - terabyte
	TB
	// PB - petabyte
	PB
	// EB - exabyte
	EB
	// ZB - zettabyte
	ZB
	// YB - yottabyte
	YB
)

func (b ByteSize) String() string {
	switch {
	case b >= YB:
		return fmt.Sprintf("%.2fYB", b/YB)
	case b >= ZB:
		return fmt.Sprintf("%.2fZB", b/ZB)
	case b >= EB:
		return fmt.Sprintf("%.2fEB", b/EB)
	case b >= PB:
		return fmt.Sprintf("%.2fPB", b/PB)
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.2fB", b)
}

// LogLevel type specifies possible log levels.
type LogLevel int

const (
	// TRACE priority
	TRACE LogLevel = iota
	// DEBUG priority
	DEBUG
	// INFO priority
	INFO
	// NOTICE priority
	NOTICE
	// WARN priority
	WARN
	// ERROR priority
	ERROR
	// CRIT priority
	CRIT
	// ALERT priority
	ALERT
	// FATAL priority
	FATAL
)

var logLevelMap = map[string]LogLevel{
	"TRACE":  TRACE,
	"DEBUG":  DEBUG,
	"INFO":   INFO,
	"NOTICE": NOTICE,
	"WARN":   WARN,
	"ERROR":  ERROR,
	"CRIT":   CRIT,
	"ALERT":  ALERT,
	"FATAL":  FATAL,
}

func (l LogLevel) String() string {
	switch l {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case NOTICE:
		return "NOTICE"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case CRIT:
		return "CRIT"
	case ALERT:
		return "ALERT"
	case FATAL:
		return "FATAL"
	}
	return "NOTSET"
}

// Personality allows to set the architecture for the container.
type Personality int64

const (
	X86    Personality = 0x0008
	X86_64             = 0x0000
)

const (
	MIGRATE_PRE_DUMP      = 0
	MIGRATE_DUMP          = 1
	MIGRATE_RESTORE       = 2
	MIGRATE_FEATURE_CHECK = 3
)

type CriuFeatures uint64

const (
	FEATURE_MEM_TRACK CriuFeatures = 1 << iota
	FEATURE_LAZY_PAGES
)
