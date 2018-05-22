// Copyright © 2013, 2014, The Go-LXC Authors. All rights reserved.
// Use of this source code is governed by a LGPLv2.1
// license that can be found in the LICENSE file.

// +build linux,cgo

package lxc

// #include <lxc/lxccontainer.h>
// #include <lxc/version.h>
// #include "lxc-binding.h"
import "C"

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Container struct
type Container struct {
	mu        sync.RWMutex
	container *C.struct_lxc_container

	verbosity Verbosity
}

// Snapshot struct
type Snapshot struct {
	Name        string
	CommentPath string
	Timestamp   string
	Path        string
}

const (
	isDefined = 1 << iota
	isNotDefined
	isRunning
	isNotRunning
	isPrivileged
	isUnprivileged
	isGreaterEqualThanLXC11
	isGreaterEqualThanLXC20
)

func (c *Container) makeSure(flags int) error {
	if flags&isDefined != 0 && !c.defined() {
		return fmt.Errorf("%s: %q", ErrNotDefined, c.name())
	}

	if flags&isNotDefined != 0 && c.defined() {
		return fmt.Errorf("%s: %q", ErrAlreadyDefined, c.name())
	}

	if flags&isRunning != 0 && !c.running() {
		return fmt.Errorf("%s: %q", ErrNotRunning, c.name())
	}

	if flags&isNotRunning != 0 && c.running() {
		return fmt.Errorf("%s: %q", ErrAlreadyRunning, c.name())
	}

	if flags&isPrivileged != 0 && os.Geteuid() != 0 {
		return ErrMethodNotAllowed
	}

	if flags&isGreaterEqualThanLXC11 != 0 && !VersionAtLeast(1, 1, 0) {
		return ErrNotSupported
	}

	if flags&isGreaterEqualThanLXC20 != 0 && !VersionAtLeast(2, 0, 0) {
		return ErrNotSupported
	}

	return nil
}

func (c *Container) cgroupItemAsByteSize(filename string, missing error) (ByteSize, error) {
	size, err := strconv.ParseFloat(c.cgroupItem(filename)[0], 64)
	if err != nil {
		return -1, missing
	}
	return ByteSize(size), nil
}

func (c *Container) setCgroupItemWithByteSize(filename string, limit ByteSize, missing error) error {
	if err := c.setCgroupItem(filename, fmt.Sprintf("%.f", limit)); err != nil {
		return missing
	}
	return nil
}

func (c *Container) name() string {
	return C.GoString(c.container.name)
}

// Name returns the name of the container.
func (c *Container) Name() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.name()
}

// String returns the string represantation of container.
func (c *Container) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return path.Join(c.configPath(), c.name())
}

// Caller needs to hold the lock
func (c *Container) defined() bool {
	return bool(C.go_lxc_defined(c.container))
}

// Defined returns true if the container is already defined.
func (c *Container) Defined() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.defined()
}

// Caller needs to hold the lock
func (c *Container) running() bool {
	return bool(C.go_lxc_running(c.container))
}

// Running returns true if the container is already running.
func (c *Container) Running() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.running()
}

// Controllable returns true if the caller can control the container.
func (c *Container) Controllable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return bool(C.go_lxc_may_control(c.container))
}

// CreateSnapshot creates a new snapshot.
func (c *Container) CreateSnapshot() (*Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isDefined | isNotRunning); err != nil {
		return nil, err
	}

	ret := int(C.go_lxc_snapshot(c.container))
	if ret < 0 {
		return nil, ErrCreateSnapshotFailed
	}
	return &Snapshot{Name: fmt.Sprintf("snap%d", ret)}, nil
}

// RestoreSnapshot creates a new container based on a snapshot.
func (c *Container) RestoreSnapshot(snapshot Snapshot, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isDefined); err != nil {
		return err
	}

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	csnapname := C.CString(snapshot.Name)
	defer C.free(unsafe.Pointer(csnapname))

	if !bool(C.go_lxc_snapshot_restore(c.container, csnapname, cname)) {
		return ErrRestoreSnapshotFailed
	}
	return nil
}

// DestroySnapshot destroys the specified snapshot.
func (c *Container) DestroySnapshot(snapshot Snapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isDefined); err != nil {
		return err
	}

	csnapname := C.CString(snapshot.Name)
	defer C.free(unsafe.Pointer(csnapname))

	if !bool(C.go_lxc_snapshot_destroy(c.container, csnapname)) {
		return ErrDestroySnapshotFailed
	}
	return nil
}

// DestroyAllSnapshots destroys all the snapshot.
func (c *Container) DestroyAllSnapshots() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isDefined | isGreaterEqualThanLXC11); err != nil {
		return err
	}

	if !bool(C.go_lxc_snapshot_destroy_all(c.container)) {
		return ErrDestroyAllSnapshotsFailed
	}
	return nil
}

// Snapshots returns the list of container snapshots.
func (c *Container) Snapshots() ([]Snapshot, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isDefined); err != nil {
		return nil, err
	}

	var csnapshots *C.struct_lxc_snapshot

	size := int(C.go_lxc_snapshot_list(c.container, &csnapshots))
	defer freeSnapshots(csnapshots, size)

	if size < 1 {
		return nil, ErrNoSnapshot
	}

	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(csnapshots)),
		Len:  size,
		Cap:  size,
	}
	gosnapshots := *(*[]C.struct_lxc_snapshot)(unsafe.Pointer(&hdr))

	snapshots := make([]Snapshot, size, size)
	for i := 0; i < size; i++ {
		snapshots[i] = Snapshot{
			Name:        C.GoString(gosnapshots[i].name),
			Timestamp:   C.GoString(gosnapshots[i].timestamp),
			CommentPath: C.GoString(gosnapshots[i].comment_pathname),
			Path:        C.GoString(gosnapshots[i].lxcpath),
		}
	}

	return snapshots, nil
}

// Caller needs to hold the lock
func (c *Container) state() State {
	return StateMap[C.GoString(C.go_lxc_state(c.container))]
}

// State returns the state of the container.
func (c *Container) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.state()
}

// InitPid returns the process ID of the container's init process
// seen from outside the container.
func (c *Container) InitPid() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return int(C.go_lxc_init_pid(c.container))
}

// Daemonize returns true if the container wished to be daemonized.
func (c *Container) Daemonize() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return bool(c.container.daemonize)
}

// WantDaemonize determines if the container wants to run daemonized.
func (c *Container) WantDaemonize(state bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !bool(C.go_lxc_want_daemonize(c.container, C.bool(state))) {
		return ErrDaemonizeFailed
	}
	return nil
}

// WantCloseAllFds determines whether container wishes all file descriptors
// to be closed on startup.
func (c *Container) WantCloseAllFds(state bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !bool(C.go_lxc_want_close_all_fds(c.container, C.bool(state))) {
		return ErrCloseAllFdsFailed
	}
	return nil
}

// SetVerbosity sets the verbosity level of some API calls
func (c *Container) SetVerbosity(verbosity Verbosity) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.verbosity = verbosity
}

// Freeze freezes the running container.
func (c *Container) Freeze() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// check the state using lockless version
	if c.state() == FROZEN {
		return ErrAlreadyFrozen
	}

	if !bool(C.go_lxc_freeze(c.container)) {
		return ErrFreezeFailed
	}

	return nil
}

// Unfreeze thaws the frozen container.
func (c *Container) Unfreeze() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	// check the state using lockless version
	if c.state() != FROZEN {
		return ErrNotFrozen
	}

	if !bool(C.go_lxc_unfreeze(c.container)) {
		return ErrUnfreezeFailed
	}

	return nil
}

// Create creates the container using given TemplateOptions
func (c *Container) Create(options TemplateOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// FIXME: Support bdev_specs
	//
	// bdev_specs:
	// zfs requires zfsroot
	// lvm requires lvname/vgname/thinpool as well as fstype and fssize
	// btrfs requires nothing
	// dir requires nothing
	if err := c.makeSure(isNotDefined); err != nil {
		return err
	}

	// use download template if not set
	if options.Template == "" {
		options.Template = "download"
	}

	// use Directory backend if not set
	if options.Backend == 0 {
		options.Backend = Directory
	}

	var args []string
	if options.Template == "download" {
		// required parameters
		if options.Distro == "" || options.Release == "" || options.Arch == "" {
			return ErrInsufficientNumberOfArguments
		}
		args = append(args, "--dist", options.Distro, "--release", options.Release, "--arch", options.Arch)

		// optional arguments
		if options.Variant != "" {
			args = append(args, "--variant", options.Variant)
		}
		if options.Server != "" {
			args = append(args, "--server", options.Server)
		}
		if options.KeyID != "" {
			args = append(args, "--keyid", options.KeyID)
		}
		if options.KeyServer != "" {
			args = append(args, "--keyserver", options.KeyServer)
		}
		if options.DisableGPGValidation {
			args = append(args, "--no-validate")
		}
		if options.FlushCache {
			args = append(args, "--flush-cache")
		}
		if options.ForceCache {
			args = append(args, "--force-cache")
		}
	} else {
		// optional arguments
		if options.Release != "" {
			args = append(args, "--release", options.Release)
		}
		if options.Arch != "" {
			args = append(args, "--arch", options.Arch)
		}
		if options.FlushCache {
			args = append(args, "--flush-cache")
		}
	}

	if options.ExtraArgs != nil {
		args = append(args, options.ExtraArgs...)
	}

	ctemplate := C.CString(options.Template)
	defer C.free(unsafe.Pointer(ctemplate))

	cbackend := C.CString(options.Backend.String())
	defer C.free(unsafe.Pointer(cbackend))

	ret := false
	if args != nil {
		cargs := makeNullTerminatedArgs(args)
		if cargs == nil {
			return ErrAllocationFailed
		}
		defer freeNullTerminatedArgs(cargs, len(args))

		ret = bool(C.go_lxc_create(c.container, ctemplate, cbackend, C.int(c.verbosity), cargs))
	} else {
		ret = bool(C.go_lxc_create(c.container, ctemplate, cbackend, C.int(c.verbosity), nil))
	}

	if !ret {
		return ErrCreateFailed
	}
	return nil
}

// Start starts the container.
func (c *Container) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isNotRunning); err != nil {
		return err
	}

	if !bool(C.go_lxc_start(c.container, 0, nil)) {
		return ErrStartFailed
	}
	return nil
}

// StartWithArgs starts the container using given arguments.
func (c *Container) StartWithArgs(args []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isNotRunning); err != nil {
		return err
	}

	if !bool(C.go_lxc_start(c.container, 0, makeNullTerminatedArgs(args))) {
		return ErrStartFailed
	}
	return nil
}

// StartExecute starts a container. It runs a minimal init as PID 1 and the
// requested program as the second process.
func (c *Container) StartExecute(args []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isNotRunning); err != nil {
		return err
	}

	if !bool(C.go_lxc_start(c.container, 1, makeNullTerminatedArgs(args))) {
		return ErrStartFailed
	}

	return nil
}

// Execute executes the given command in a temporary container.
func (c *Container) Execute(args ...string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isNotDefined); err != nil {
		return nil, err
	}

	os.MkdirAll(filepath.Join(c.configPath(), c.name()), 0700)
	c.saveConfigFile(filepath.Join(c.configPath(), c.name(), "config"))
	defer os.RemoveAll(filepath.Join(c.configPath(), c.name()))

	cargs := []string{"lxc-execute", "-n", c.name(), "-P", c.configPath(), "--"}
	cargs = append(cargs, args...)

	// FIXME: Go runtime and src/lxc/start.c signal_handler are not playing nice together so use lxc-execute for now
	// go-nuts thread: https://groups.google.com/forum/#!msg/golang-nuts/h9GbvfYv83w/5Ly_jvOr86wJ
	output, err := exec.Command(cargs[0], cargs[1:]...).CombinedOutput()
	if err != nil {
		return nil, ErrExecuteFailed
	}

	return output, nil
}

// Stop stops the container.
func (c *Container) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	if !bool(C.go_lxc_stop(c.container)) {
		return ErrStopFailed
	}
	return nil
}

// Reboot reboots the container.
func (c *Container) Reboot() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	if !bool(C.go_lxc_reboot(c.container)) {
		return ErrRebootFailed
	}
	return nil
}

// Shutdown shuts down the container.
func (c *Container) Shutdown(timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	if !bool(C.go_lxc_shutdown(c.container, C.int(timeout.Seconds()))) {
		return ErrShutdownFailed
	}
	return nil
}

// Destroy destroys the container.
func (c *Container) Destroy() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isDefined | isNotRunning); err != nil {
		return err
	}

	if !bool(C.go_lxc_destroy(c.container)) {
		return ErrDestroyFailed
	}
	return nil
}

// DestroyWithAllSnapshots destroys the container and its snapshots
func (c *Container) DestroyWithAllSnapshots() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isDefined | isNotRunning | isGreaterEqualThanLXC11); err != nil {
		return err
	}

	if !bool(C.go_lxc_destroy_with_snapshots(c.container)) {
		return ErrDestroyWithAllSnapshotsFailed
	}
	return nil
}

// Clone clones the container using given arguments with specified backend.
func (c *Container) Clone(name string, options CloneOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// FIXME: bdevdata, newsize and hookargs
	//
	// bdevdata:
	// zfs requires zfsroot
	// lvm requires lvname/vgname/thinpool as well as fstype and fssize
	// btrfs requires nothing
	// dir requires nothing
	//
	// newsize: for blockdev-backed backingstores
	//
	// hookargs: additional arguments to pass to the clone hook script
	if err := c.makeSure(isDefined | isNotRunning); err != nil {
		return err
	}

	// use Directory backend if not set
	if options.Backend == 0 {
		options.Backend = Directory
	}

	var flags int
	if options.KeepName {
		flags |= C.LXC_CLONE_KEEPNAME
	}
	if options.KeepMAC {
		flags |= C.LXC_CLONE_KEEPMACADDR
	}
	if options.Snapshot {
		flags |= C.LXC_CLONE_SNAPSHOT
	}

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	cbackend := C.CString(options.Backend.String())
	defer C.free(unsafe.Pointer(cbackend))

	if options.ConfigPath != "" {
		clxcpath := C.CString(options.ConfigPath)
		defer C.free(unsafe.Pointer(clxcpath))

		if !bool(C.go_lxc_clone(c.container, cname, clxcpath, C.int(flags), cbackend)) {
			return ErrCloneFailed
		}
	} else {
		if !bool(C.go_lxc_clone(c.container, cname, nil, C.int(flags), cbackend)) {
			return ErrCloneFailed
		}
	}
	return nil
}

// Rename renames the container.
func (c *Container) Rename(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isDefined | isNotRunning); err != nil {
		return err
	}

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	if !bool(C.go_lxc_rename(c.container, cname)) {
		return ErrRenameFailed
	}
	return nil
}

// Wait waits for container to reach a particular state.
func (c *Container) Wait(state State, timeout time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	cstate := C.CString(state.String())
	defer C.free(unsafe.Pointer(cstate))

	return bool(C.go_lxc_wait(c.container, cstate, C.int(timeout.Seconds())))
}

// ConfigFileName returns the container's configuration file's name.
func (c *Container) ConfigFileName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// allocated in lxc.c
	configFileName := C.go_lxc_config_file_name(c.container)
	defer C.free(unsafe.Pointer(configFileName))

	return C.GoString(configFileName)
}

func (c *Container) configItem(key string) []string {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	// allocated in lxc.c
	configItem := C.go_lxc_get_config_item(c.container, ckey)
	defer C.free(unsafe.Pointer(configItem))

	ret := strings.TrimSpace(C.GoString(configItem))
	return strings.Split(ret, "\n")
}

// ConfigItem returns the value of the given config item.
func (c *Container) ConfigItem(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.configItem(key)
}

func (c *Container) setConfigItem(key string, value string) error {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))

	if !bool(C.go_lxc_set_config_item(c.container, ckey, cvalue)) {
		return ErrSettingConfigItemFailed
	}
	return nil
}

// SetConfigItem sets the value of the given config item.
func (c *Container) SetConfigItem(key string, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.setConfigItem(key, value)
}

func (c *Container) runningConfigItem(key string) []string {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	// allocated in lxc.c
	configItem := C.go_lxc_get_running_config_item(c.container, ckey)
	defer C.free(unsafe.Pointer(configItem))

	ret := strings.TrimSpace(C.GoString(configItem))
	return strings.Split(ret, "\n")
}

// RunningConfigItem returns the value of the given config item.
func (c *Container) RunningConfigItem(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.runningConfigItem(key)
}

func (c *Container) cgroupItem(key string) []string {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	// allocated in lxc.c
	cgroupItem := C.go_lxc_get_cgroup_item(c.container, ckey)
	defer C.free(unsafe.Pointer(cgroupItem))

	ret := strings.TrimSpace(C.GoString(cgroupItem))
	return strings.Split(ret, "\n")
}

func (c *Container) setCgroupItem(key string, value string) error {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))

	if !bool(C.go_lxc_set_cgroup_item(c.container, ckey, cvalue)) {
		return ErrSettingCgroupItemFailed
	}
	return nil
}

// CgroupItem returns the value of the given cgroup subsystem value.
func (c *Container) CgroupItem(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cgroupItem(key)
}

// SetCgroupItem sets the value of given cgroup subsystem value.
func (c *Container) SetCgroupItem(key string, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.setCgroupItem(key, value)
}

// ClearConfig completely clears the containers in-memory configuration.
func (c *Container) ClearConfig() {
	c.mu.Lock()
	defer c.mu.Unlock()

	C.go_lxc_clear_config(c.container)
}

// ClearConfigItem clears the value of given config item.
func (c *Container) ClearConfigItem(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	if !bool(C.go_lxc_clear_config_item(c.container, ckey)) {
		return ErrClearingConfigItemFailed
	}
	return nil
}

// ConfigKeys returns the names of the config items.
func (c *Container) ConfigKeys(key ...string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var keys *_Ctype_char

	if key != nil && len(key) == 1 {
		ckey := C.CString(key[0])
		defer C.free(unsafe.Pointer(ckey))

		// allocated in lxc.c
		keys = C.go_lxc_get_keys(c.container, ckey)
		defer C.free(unsafe.Pointer(keys))
	} else {
		// allocated in lxc.c
		keys = C.go_lxc_get_keys(c.container, nil)
		defer C.free(unsafe.Pointer(keys))
	}
	ret := strings.TrimSpace(C.GoString(keys))
	return strings.Split(ret, "\n")
}

// LoadConfigFile loads the configuration file from given path.
func (c *Container) LoadConfigFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	if !bool(C.go_lxc_load_config(c.container, cpath)) {
		return ErrLoadConfigFailed
	}
	return nil
}

func (c *Container) saveConfigFile(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	if !bool(C.go_lxc_save_config(c.container, cpath)) {
		return ErrSaveConfigFailed
	}
	return nil
}

// SaveConfigFile saves the configuration file to given path.
func (c *Container) SaveConfigFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.saveConfigFile(path)
}

func (c *Container) configPath() string {
	return C.GoString(C.go_lxc_get_config_path(c.container))
}

// ConfigPath returns the configuration file's path.
func (c *Container) ConfigPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.configPath()
}

// SetConfigPath sets the configuration file's path.
func (c *Container) SetConfigPath(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	if !bool(C.go_lxc_set_config_path(c.container, cpath)) {
		return ErrSettingConfigPathFailed
	}
	return nil
}

// MemoryUsage returns memory usage of the container in bytes.
func (c *Container) MemoryUsage() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	return c.cgroupItemAsByteSize("memory.usage_in_bytes", ErrMemLimit)
}

// MemoryLimit returns memory limit of the container in bytes.
func (c *Container) MemoryLimit() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	return c.cgroupItemAsByteSize("memory.limit_in_bytes", ErrMemLimit)
}

// SetMemoryLimit sets memory limit of the container in bytes.
func (c *Container) SetMemoryLimit(limit ByteSize) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	return c.setCgroupItemWithByteSize("memory.limit_in_bytes", limit, ErrSettingMemoryLimitFailed)
}

// SoftMemoryLimit returns soft memory limit of the container in bytes.
func (c *Container) SoftMemoryLimit() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	return c.cgroupItemAsByteSize("memory.soft_limit_in_bytes", ErrSoftMemLimit)
}

// SetSoftMemoryLimit sets soft  memory limit of the container in bytes.
func (c *Container) SetSoftMemoryLimit(limit ByteSize) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	return c.setCgroupItemWithByteSize("memory.soft_limit_in_bytes", limit, ErrSettingSoftMemoryLimitFailed)
}

// KernelMemoryUsage returns current kernel memory allocation of the container in bytes.
func (c *Container) KernelMemoryUsage() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	return c.cgroupItemAsByteSize("memory.kmem.usage_in_bytes", ErrKMemLimit)
}

// KernelMemoryLimit returns kernel memory limit of the container in bytes.
func (c *Container) KernelMemoryLimit() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	return c.cgroupItemAsByteSize("memory.kmem.usage_in_bytes", ErrKMemLimit)
}

// SetKernelMemoryLimit sets kernel memory limit of the container in bytes.
func (c *Container) SetKernelMemoryLimit(limit ByteSize) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	return c.setCgroupItemWithByteSize("memory.kmem.limit_in_bytes", limit, ErrSettingKMemoryLimitFailed)
}

// MemorySwapUsage returns memory+swap usage of the container in bytes.
func (c *Container) MemorySwapUsage() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	return c.cgroupItemAsByteSize("memory.memsw.usage_in_bytes", ErrMemorySwapLimit)
}

// MemorySwapLimit returns the memory+swap limit of the container in bytes.
func (c *Container) MemorySwapLimit() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	return c.cgroupItemAsByteSize("memory.memsw.limit_in_bytes", ErrMemorySwapLimit)
}

// SetMemorySwapLimit sets memory+swap limit of the container in bytes.
func (c *Container) SetMemorySwapLimit(limit ByteSize) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	return c.setCgroupItemWithByteSize("memory.memsw.limit_in_bytes", limit, ErrSettingMemorySwapLimitFailed)
}

// BlkioUsage returns number of bytes transferred to/from the disk by the container.
func (c *Container) BlkioUsage() (ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	ioServiceBytes := c.cgroupItem("blkio.throttle.io_service_bytes")
	if ioServiceBytes[0] == "" {
		return 0, nil
	}

	for _, v := range ioServiceBytes {
		b := strings.Split(v, " ")
		if b[0] == "Total" {
			blkioUsed, err := strconv.ParseFloat(b[1], 64)
			if err != nil {
				return -1, err
			}
			return ByteSize(blkioUsed), nil
		}
	}
	return -1, ErrBlkioUsage
}

// CPUTime returns the total CPU time (in nanoseconds) consumed by all tasks
// in this cgroup (including tasks lower in the hierarchy).
func (c *Container) CPUTime() (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	usage := c.cgroupItem("cpuacct.usage")
	if usage[0] == "" {
		return 0, nil
	}

	cpuUsage, err := strconv.ParseInt(usage[0], 10, 64)
	if err != nil {
		return -1, err
	}
	return time.Duration(cpuUsage), nil
}

// CPUTimePerCPU returns the CPU time (in nanoseconds) consumed on each CPU by
// all tasks in this cgroup (including tasks lower in the hierarchy).
func (c *Container) CPUTimePerCPU() (map[int]time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	usagePerCPU := c.cgroupItem("cpuacct.usage_percpu")
	if usagePerCPU[0] == "" {
		return map[int]time.Duration{0: 0}, nil
	}

	cpuTimes := make(map[int]time.Duration)
	for i, v := range strings.Split(usagePerCPU[0], " ") {
		cpuUsage, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		cpuTimes[i] = time.Duration(cpuUsage)
	}
	return cpuTimes, nil
}

// CPUStats returns the number of CPU cycles (in the units defined by USER_HZ on the system)
// consumed by tasks in this cgroup and its children in both user mode and system (kernel) mode.
func (c *Container) CPUStats() (map[string]int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	stat := c.cgroupItem("cpuacct.stat")
	if stat[0] == "" {
		return map[string]int64{"user": 0, "system": 0}, nil
	}

	user, err := strconv.ParseInt(strings.Split(stat[0], "user ")[1], 10, 64)
	if err != nil {
		return nil, err
	}
	system, err := strconv.ParseInt(strings.Split(stat[1], "system ")[1], 10, 64)
	if err != nil {
		return nil, err
	}

	return map[string]int64{"user": user, "system": system}, nil
}

// ConsoleFd allocates a console tty from container
// ttynum: tty number to attempt to allocate or -1 to allocate the first available tty
//
// Returns "ttyfd" on success, -1 on failure. The returned "ttyfd" is
// used to keep the tty allocated. The caller should close "ttyfd" to
// indicate that it is done with the allocated console so that it can
// be allocated by another caller.
func (c *Container) ConsoleFd(ttynum int) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// FIXME: Make idiomatic
	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	ret := int(C.go_lxc_console_getfd(c.container, C.int(ttynum)))
	if ret < 0 {
		return -1, ErrAttachFailed
	}
	return ret, nil
}

// Console allocates and runs a console tty from container
//
// This function will not return until the console has been exited by the user.
func (c *Container) Console(options ConsoleOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	ret := bool(C.go_lxc_console(c.container,
		C.int(options.Tty),
		C.int(options.StdinFd),
		C.int(options.StdoutFd),
		C.int(options.StderrFd),
		C.int(options.EscapeCharacter)))

	if !ret {
		return ErrAttachFailed
	}
	return nil
}

// AttachShell attaches a shell to the container.
// It clears all environment variables before attaching.
func (c *Container) AttachShell(options AttachOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning); err != nil {
		return err
	}

	cenv := makeNullTerminatedArgs(options.Env)
	if cenv == nil {
		return ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cenv, len(options.Env))

	cenvToKeep := makeNullTerminatedArgs(options.EnvToKeep)
	if cenvToKeep == nil {
		return ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cenvToKeep, len(options.EnvToKeep))

	cwd := C.CString(options.Cwd)
	defer C.free(unsafe.Pointer(cwd))

	ret := int(C.go_lxc_attach(c.container,
		C.bool(options.ClearEnv),
		C.int(options.Namespaces),
		C.long(options.Arch),
		C.uid_t(options.UID),
		C.gid_t(options.GID),
		C.int(options.StdinFd),
		C.int(options.StdoutFd),
		C.int(options.StderrFd),
		cwd,
		cenv,
		cenvToKeep,
	))
	if ret < 0 {
		return ErrAttachFailed
	}
	return nil
}

func (c *Container) runCommandStatus(args []string, options AttachOptions) (int, error) {
	if len(args) == 0 {
		return -1, ErrInsufficientNumberOfArguments
	}

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	cargs := makeNullTerminatedArgs(args)
	if cargs == nil {
		return -1, ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cargs, len(args))

	cenv := makeNullTerminatedArgs(options.Env)
	if cenv == nil {
		return -1, ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cenv, len(options.Env))

	cenvToKeep := makeNullTerminatedArgs(options.EnvToKeep)
	if cenvToKeep == nil {
		return -1, ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cenvToKeep, len(options.EnvToKeep))

	cwd := C.CString(options.Cwd)
	defer C.free(unsafe.Pointer(cwd))

	ret := int(C.go_lxc_attach_run_wait(
		c.container,
		C.bool(options.ClearEnv),
		C.int(options.Namespaces),
		C.long(options.Arch),
		C.uid_t(options.UID),
		C.gid_t(options.GID),
		C.int(options.StdinFd),
		C.int(options.StdoutFd),
		C.int(options.StderrFd),
		cwd,
		cenv,
		cenvToKeep,
		cargs,
	))

	return ret, nil
}

// RunCommandStatus attachs a shell and runs the command within the container.
// The process will wait for the command to finish and return the result of
// waitpid(), i.e. the process' exit status. An error is returned only when
// invocation of the command completely fails.
func (c *Container) RunCommandStatus(args []string, options AttachOptions) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.runCommandStatus(args, options)
}

// RunCommandNoWait runs the given command and returns without waiting it to finish.
func (c *Container) RunCommandNoWait(args []string, options AttachOptions) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(args) == 0 {
		return -1, ErrInsufficientNumberOfArguments
	}

	if err := c.makeSure(isRunning); err != nil {
		return -1, err
	}

	cargs := makeNullTerminatedArgs(args)
	if cargs == nil {
		return -1, ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cargs, len(args))

	cenv := makeNullTerminatedArgs(options.Env)
	if cenv == nil {
		return -1, ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cenv, len(options.Env))

	cenvToKeep := makeNullTerminatedArgs(options.EnvToKeep)
	if cenvToKeep == nil {
		return -1, ErrAllocationFailed
	}
	defer freeNullTerminatedArgs(cenvToKeep, len(options.EnvToKeep))

	cwd := C.CString(options.Cwd)
	defer C.free(unsafe.Pointer(cwd))

	var attachedPid C.pid_t
	ret := int(C.go_lxc_attach_no_wait(
		c.container,
		C.bool(options.ClearEnv),
		C.int(options.Namespaces),
		C.long(options.Arch),
		C.uid_t(options.UID),
		C.gid_t(options.GID),
		C.int(options.StdinFd),
		C.int(options.StdoutFd),
		C.int(options.StderrFd),
		cwd,
		cenv,
		cenvToKeep,
		cargs,
		&attachedPid,
	))

	if ret < 0 {
		return -1, ErrAttachFailed
	}

	return int(attachedPid), nil
}

// RunCommand attachs a shell and runs the command within the container.
// The process will wait for the command to finish and return a success status. An error
// is returned only when invocation of the command completely fails.
func (c *Container) RunCommand(args []string, options AttachOptions) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ret, err := c.runCommandStatus(args, options)
	if err != nil {
		return false, err
	}
	if ret < 0 {
		return false, ErrAttachFailed
	}
	return ret == 0, nil
}

// Interfaces returns the names of the network interfaces.
func (c *Container) Interfaces() ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	result := C.go_lxc_get_interfaces(c.container)
	if result == nil {
		return nil, ErrInterfaces
	}
	return convertArgs(result), nil
}

// InterfaceStats returns the stats about container's network interfaces
func (c *Container) InterfaceStats() (map[string]map[string]ByteSize, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	var interfaceName string

	statistics := make(map[string]map[string]ByteSize)

	netPrefix := "lxc.net"
	if !VersionAtLeast(2, 1, 0) {
		netPrefix = "lxc.network"
	}

	for i := 0; i < len(c.configItem(netPrefix)); i++ {
		interfaceType := c.runningConfigItem(fmt.Sprintf("%s.%d.type", netPrefix, i))
		if interfaceType == nil {
			continue
		}

		if interfaceType[0] == "veth" {
			interfaceName = c.runningConfigItem(fmt.Sprintf("%s.%d.veth.pair", netPrefix, i))[0]
		} else {
			interfaceName = c.runningConfigItem(fmt.Sprintf("%s.%d.link", netPrefix, i))[0]
		}

		for _, v := range []string{"rx", "tx"} {
			/* tx and rx are reversed from the host vs container */
			content, err := ioutil.ReadFile(fmt.Sprintf("/sys/class/net/%s/statistics/%s_bytes", interfaceName, v))
			if err != nil {
				return nil, err
			}

			bytes, err := strconv.ParseInt(strings.Split(string(content), "\n")[0], 10, 64)
			if err != nil {
				return nil, err
			}

			if statistics[interfaceName] == nil {
				statistics[interfaceName] = make(map[string]ByteSize)
			}
			statistics[interfaceName][v] = ByteSize(bytes)
		}
	}

	return statistics, nil
}

// IPAddress returns the IP address of the given network interface.
func (c *Container) IPAddress(interfaceName string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	cinterface := C.CString(interfaceName)
	defer C.free(unsafe.Pointer(cinterface))

	result := C.go_lxc_get_ips(c.container, cinterface, nil, 0)
	if result == nil {
		return nil, ErrIPAddress
	}
	return convertArgs(result), nil
}

// IPv4Address returns the IPv4 address of the given network interface.
func (c *Container) IPv4Address(interfaceName string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	cinterface := C.CString(interfaceName)
	defer C.free(unsafe.Pointer(cinterface))

	cfamily := C.CString("inet")
	defer C.free(unsafe.Pointer(cfamily))

	result := C.go_lxc_get_ips(c.container, cinterface, cfamily, 0)
	if result == nil {
		return nil, ErrIPv4Addresses
	}
	return convertArgs(result), nil
}

// IPv6Address returns the IPv6 address of the given network interface.
func (c *Container) IPv6Address(interfaceName string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	cinterface := C.CString(interfaceName)
	defer C.free(unsafe.Pointer(cinterface))

	cfamily := C.CString("inet6")
	defer C.free(unsafe.Pointer(cfamily))

	result := C.go_lxc_get_ips(c.container, cinterface, cfamily, 0)
	if result == nil {
		return nil, ErrIPv6Addresses
	}
	return convertArgs(result), nil
}

// WaitIPAddresses waits until IPAddresses call returns something or time outs
func (c *Container) WaitIPAddresses(timeout time.Duration) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	for {
		if result, err := c.ipAddresses(); err == nil && len(result) > 0 {
			return result, nil
		}
		// Python API sleeps 1 second as well
		time.Sleep(1 * time.Second)

		if time.Since(now) >= timeout {
			return nil, ErrIPAddresses
		}
	}
}

func (c *Container) ipAddresses() ([]string, error) {
	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	result := C.go_lxc_get_ips(c.container, nil, nil, 0)
	if result == nil {
		return nil, ErrIPAddresses
	}
	return convertArgs(result), nil

}

// IPAddresses returns all IP addresses.
func (c *Container) IPAddresses() ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.ipAddresses()
}

// IPv4Addresses returns all IPv4 addresses.
func (c *Container) IPv4Addresses() ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	cfamily := C.CString("inet")
	defer C.free(unsafe.Pointer(cfamily))

	result := C.go_lxc_get_ips(c.container, nil, cfamily, 0)
	if result == nil {
		return nil, ErrIPv4Addresses
	}
	return convertArgs(result), nil
}

// IPv6Addresses returns all IPv6 addresses.
func (c *Container) IPv6Addresses() ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.makeSure(isRunning); err != nil {
		return nil, err
	}

	cfamily := C.CString("inet6")
	defer C.free(unsafe.Pointer(cfamily))

	result := C.go_lxc_get_ips(c.container, nil, cfamily, 0)
	if result == nil {
		return nil, ErrIPv6Addresses
	}
	return convertArgs(result), nil
}

// LogFile returns the name of the logfile.
func (c *Container) LogFile() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if VersionAtLeast(2, 1, 0) {
		return c.configItem("lxc.log.file")[0]
	}

	return c.configItem("lxc.logfile")[0]
}

// SetLogFile sets the name of the logfile.
func (c *Container) SetLogFile(filename string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	if VersionAtLeast(2, 1, 0) {
		err = c.setConfigItem("lxc.log.file", filename)
	} else {
		err = c.setConfigItem("lxc.logfile", filename)
	}
	if err != nil {
		return err
	}

	return nil
}

// LogLevel returns the level of the logfile.
func (c *Container) LogLevel() LogLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if VersionAtLeast(2, 1, 0) {
		return logLevelMap[c.configItem("lxc.log.level")[0]]
	}

	return logLevelMap[c.configItem("lxc.loglevel")[0]]
}

// SetLogLevel sets the level of the logfile.
func (c *Container) SetLogLevel(level LogLevel) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	if VersionAtLeast(2, 1, 0) {
		err = c.setConfigItem("lxc.log.level", level.String())
	} else {
		err = c.setConfigItem("lxc.loglevel", level.String())
	}
	if err != nil {
		return err
	}
	return nil
}

// AddDeviceNode adds specified device to the container.
func (c *Container) AddDeviceNode(source string, destination ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning | isPrivileged); err != nil {
		return err
	}

	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	if destination != nil && len(destination) == 1 {
		cdestination := C.CString(destination[0])
		defer C.free(unsafe.Pointer(cdestination))

		if !bool(C.go_lxc_add_device_node(c.container, csource, cdestination)) {
			return ErrAddDeviceNodeFailed
		}
		return nil
	}

	if !bool(C.go_lxc_add_device_node(c.container, csource, nil)) {
		return ErrAddDeviceNodeFailed
	}
	return nil
}

// RemoveDeviceNode removes the specified device from the container.
func (c *Container) RemoveDeviceNode(source string, destination ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning | isPrivileged); err != nil {
		return err
	}

	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	if destination != nil && len(destination) == 1 {
		cdestination := C.CString(destination[0])
		defer C.free(unsafe.Pointer(cdestination))

		if !bool(C.go_lxc_remove_device_node(c.container, csource, cdestination)) {
			return ErrRemoveDeviceNodeFailed
		}
		return nil
	}

	if !bool(C.go_lxc_remove_device_node(c.container, csource, nil)) {
		return ErrRemoveDeviceNodeFailed
	}
	return nil
}

// Checkpoint checkpoints the container.
func (c *Container) Checkpoint(opts CheckpointOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning | isGreaterEqualThanLXC11); err != nil {
		return err
	}

	cdirectory := C.CString(opts.Directory)
	defer C.free(unsafe.Pointer(cdirectory))

	cstop := C.bool(opts.Stop)
	cverbose := C.bool(opts.Verbose)

	if !C.go_lxc_checkpoint(c.container, cdirectory, cstop, cverbose) {
		return ErrCheckpointFailed
	}
	return nil
}

// Restore restores the container from a checkpoint.
func (c *Container) Restore(opts RestoreOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isGreaterEqualThanLXC11); err != nil {
		return err
	}

	cdirectory := C.CString(opts.Directory)
	defer C.free(unsafe.Pointer(cdirectory))

	cverbose := C.bool(opts.Verbose)

	if !C.bool(C.go_lxc_restore(c.container, cdirectory, cverbose)) {
		return ErrRestoreFailed
	}
	return nil
}

// Migrate migrates the container.
func (c *Container) Migrate(cmd uint, opts MigrateOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isNotDefined | isGreaterEqualThanLXC20); err != nil {
		return err
	}

	if cmd != MIGRATE_RESTORE {
		if err := c.makeSure(isRunning); err != nil {
			return err
		}
	}

	cdirectory := C.CString(opts.Directory)
	defer C.free(unsafe.Pointer(cdirectory))

	var cpredumpdir *C.char

	if opts.PredumpDir != "" {
		cpredumpdir = C.CString(opts.PredumpDir)
		defer C.free(unsafe.Pointer(cpredumpdir))
	}

	/* Since we can't do conditional compilation here, we allocate the
	 * "extras" struct and then merge them in the C code.
	 */
	copts := C.struct_migrate_opts{
		directory:   cdirectory,
		verbose:     C.bool(opts.Verbose),
		stop:        C.bool(opts.Stop),
		predump_dir: cpredumpdir,
	}

	var cActionScript *C.char
	if opts.ActionScript != "" {
		cActionScript = C.CString(opts.ActionScript)
		defer C.free(unsafe.Pointer(cActionScript))
	}

	extras := C.struct_extra_migrate_opts{
		preserves_inodes:  C.bool(opts.PreservesInodes),
		action_script:     cActionScript,
		ghost_limit:       C.uint64_t(opts.GhostLimit),
		features_to_check: C.uint64_t(opts.FeaturesToCheck),
	}

	ret := C.int(C.go_lxc_migrate(c.container, C.uint(cmd), &copts, &extras))
	if ret != 0 {
		return fmt.Errorf("migration failed %d", ret)
	}

	return nil
}

// AttachInterface attaches specifed netdev to the container.
func (c *Container) AttachInterface(source, destination string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning | isPrivileged | isGreaterEqualThanLXC11); err != nil {
		return err
	}

	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	cdestination := C.CString(destination)
	defer C.free(unsafe.Pointer(cdestination))

	if !bool(C.go_lxc_attach_interface(c.container, csource, cdestination)) {
		return ErrAttachInterfaceFailed
	}
	return nil
}

// DetachInterface detaches specifed netdev from the container.
func (c *Container) DetachInterface(source string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning | isPrivileged | isGreaterEqualThanLXC11); err != nil {
		return err
	}

	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	if !bool(C.go_lxc_detach_interface(c.container, csource, nil)) {
		return ErrDetachInterfaceFailed
	}
	return nil
}

// DetachInterfaceRename detaches specifed netdev from the container and renames it.
func (c *Container) DetachInterfaceRename(source, target string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.makeSure(isRunning | isPrivileged | isGreaterEqualThanLXC11); err != nil {
		return err
	}

	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	ctarget := C.CString(target)
	defer C.free(unsafe.Pointer(ctarget))

	if !bool(C.go_lxc_detach_interface(c.container, csource, ctarget)) {
		return ErrDetachInterfaceFailed
	}
	return nil
}

// ConsoleLog allows to perform operations on the container's in-memory console
// buffer.
func (c *Container) ConsoleLog(opt ConsoleLogOptions) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cl := C.struct_lxc_console_log{
		clear: C.bool(opt.ClearLog),
		read:  C.bool(opt.ReadLog),
		data:  nil,
	}
	// CGO is a fickle little beast:
	// We need to manually allocate memory here that we pass to C. If we
	// were to pass a GO pointer by passing a C.uint64_t pointer we'd end in
	// the situation where we have a GO pointer that points to a GO pointer.
	// Go will freak out when this happens. So give C its own memory.
	var buf unsafe.Pointer
	buf = C.malloc(C.sizeof_uint64_t)
	if buf == nil {
		return nil, syscall.ENOMEM
	}
	defer C.free(buf)

	cl.read_max = (*C.uint64_t)(buf)
	*cl.read_max = C.uint64_t(opt.ReadMax)

	ret := C.go_lxc_console_log(c.container, &cl)
	if ret < 0 {
		return nil, syscall.Errno(-ret)
	}

	numBytes := C.int(*cl.read_max)
	if C.uint64_t(numBytes) != *cl.read_max {
		return nil, syscall.ERANGE
	}

	return C.GoBytes(unsafe.Pointer(cl.data), numBytes), nil
}

// ErrorNum returns the error_num field of the container.
func (c *Container) ErrorNum() int {
	cError := C.go_lxc_error_num(c.container)
	return int(cError)
}
