package allocrunner

import (
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/sys/unix"
)

const (
	NsRunDir = "/var/run/netns"
)

type networkManager interface {
	CreateNetwork(allocID string) (*drivers.NetworkIsolationSpec, error)
	DestroyNetwork(allocID string, spec *drivers.NetworkIsolationSpec) error
}

func (ar *allocRunner) netNSPath() string {
	return path.Join(NsRunDir, netNSName(ar.Alloc().ID))
}

func netNSName(id string) string {
	return fmt.Sprintf("nomad-%s", id)
}

type networkHook struct {
	setter   *allocNetworkIsolationSetter
	manager  networkManager
	alloc    *structs.Allocation
	spec     *drivers.NetworkIsolationSpec
	specLock sync.Mutex
	logger   hclog.Logger
}

func newNetworkHook(ns *allocNetworkIsolationSetter, logger hclog.Logger, alloc *structs.Allocation, netManager networkManager) *networkHook {
	return &networkHook{
		setter:  ns,
		alloc:   alloc,
		manager: netManager,
		logger:  logger,
	}
}

func (h *networkHook) Name() string {
	return "network"
}

func (h *networkHook) Prerun() error {
	h.specLock.Lock()
	defer h.specLock.Unlock()

	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	if len(tg.Networks) == 0 || tg.Networks[0].Mode == "host" || tg.Networks[0].Mode == "" {
		return nil
	}

	spec, err := h.manager.CreateNetwork(h.alloc.ID)
	if err != nil {
		return fmt.Errorf("failed to create network for alloc: %v", err)
	}

	h.spec = spec
	h.setter.SetNetworkIsolation(spec)

	return nil
}

func (h *networkHook) Postrun() error {
	h.specLock.Lock()
	defer h.specLock.Unlock()
	if h.spec == nil {
		h.logger.Debug("spec was nil")
		return nil
	}

	return h.manager.DestroyNetwork(h.alloc.ID, h.spec)
}

type defaultNetworkManager struct{}

func (_ *defaultNetworkManager) CreateNetwork(allocID string) (*drivers.NetworkIsolationSpec, error) {
	netns, err := newNS(allocID)
	if err != nil {
		return nil, err
	}

	spec := &drivers.NetworkIsolationSpec{
		Mode:   drivers.NetIsolationModeGroup,
		Path:   netns.Path(),
		Labels: make(map[string]string),
	}

	return spec, nil
}

func (_ *defaultNetworkManager) DestroyNetwork(allocID string, spec *drivers.NetworkIsolationSpec) error {
	return unmountNS(spec.Path)
}

// Creates a new persistent (bind-mounted) network namespace and returns an object
// representing that namespace, without switching to it.
func newNS(id string) (ns.NetNS, error) {

	b := make([]byte, 16)
	_, err := rand.Reader.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random netns name: %v", err)
	}

	// Create the directory for mounting network namespaces
	// This needs to be a shared mountpoint in case it is mounted in to
	// other namespaces (containers)
	err = os.MkdirAll(NsRunDir, 0755)
	if err != nil {
		return nil, err
	}

	// Remount the namespace directory shared. This will fail if it is not
	// already a mountpoint, so bind-mount it on to itself to "upgrade" it
	// to a mountpoint.
	err = unix.Mount("", NsRunDir, "none", unix.MS_SHARED|unix.MS_REC, "")
	if err != nil {
		if err != unix.EINVAL {
			return nil, fmt.Errorf("mount --make-rshared %s failed: %q", NsRunDir, err)
		}

		// Recursively remount /var/run/netns on itself. The recursive flag is
		// so that any existing netns bindmounts are carried over.
		err = unix.Mount(NsRunDir, NsRunDir, "none", unix.MS_BIND|unix.MS_REC, "")
		if err != nil {
			return nil, fmt.Errorf("mount --rbind %s %s failed: %q", NsRunDir, NsRunDir, err)
		}

		// Now we can make it shared
		err = unix.Mount("", NsRunDir, "none", unix.MS_SHARED|unix.MS_REC, "")
		if err != nil {
			return nil, fmt.Errorf("mount --make-rshared %s failed: %q", NsRunDir, err)
		}

	}

	nsName := netNSName(id)

	// create an empty file at the mount point
	nsPath := path.Join(NsRunDir, nsName)
	mountPointFd, err := os.Create(nsPath)
	if err != nil {
		return nil, err
	}
	mountPointFd.Close()

	// Ensure the mount point is cleaned up on errors; if the namespace
	// was successfully mounted this will have no effect because the file
	// is in-use
	defer os.RemoveAll(nsPath)

	var wg sync.WaitGroup
	wg.Add(1)

	// do namespace work in a dedicated goroutine, so that we can safely
	// Lock/Unlock OSThread without upsetting the lock/unlock state of
	// the caller of this function
	go (func() {
		defer wg.Done()
		runtime.LockOSThread()
		// Don't unlock. By not unlocking, golang will kill the OS thread when the
		// goroutine is done (for go1.10+)

		var origNS ns.NetNS
		origNS, err = ns.GetNS(getCurrentThreadNetNSPath())
		if err != nil {
			return
		}
		defer origNS.Close()

		// create a new netns on the current thread
		err = unix.Unshare(unix.CLONE_NEWNET)
		if err != nil {
			return
		}

		// Put this thread back to the orig ns, since it might get reused (pre go1.10)
		defer origNS.Set()

		// bind mount the netns from the current thread (from /proc) onto the
		// mount point. This causes the namespace to persist, even when there
		// are no threads in the ns.
		err = unix.Mount(getCurrentThreadNetNSPath(), nsPath, "none", unix.MS_BIND, "")
		if err != nil {
			err = fmt.Errorf("failed to bind mount ns at %s: %v", nsPath, err)
		}
	})()
	wg.Wait()

	if err != nil {
		return nil, fmt.Errorf("failed to create namespace: %v", err)
	}

	return ns.GetNS(nsPath)
}

// UnmountNS unmounts the NS held by the netns object
func unmountNS(nsPath string) error {
	// Only unmount if it's been bind-mounted (don't touch namespaces in /proc...)
	if strings.HasPrefix(nsPath, NsRunDir) {
		if err := unix.Unmount(nsPath, 0); err != nil {
			return fmt.Errorf("failed to unmount NS: at %s: %v", nsPath, err)
		}

		if err := os.Remove(nsPath); err != nil {
			return fmt.Errorf("failed to remove ns path %s: %v", nsPath, err)
		}
	}

	return nil
}

// getCurrentThreadNetNSPath copied from pkg/ns
func getCurrentThreadNetNSPath() string {
	// /proc/self/ns/net returns the namespace of the main thread, not
	// of whatever thread this goroutine is running on.  Make sure we
	// use the thread's net namespace since the thread is switching around
	return fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), unix.Gettid())
}
