// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocwatcher

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// getRemoteRetryIntv is minimum interval on which we retry
	// to fetch remote objects. We pick a value between this and 2x this.
	getRemoteRetryIntv = 30 * time.Second
)

// RPCer is the interface needed by a prevAllocWatcher to make RPC calls.
type RPCer interface {
	// RPC allows retrieving remote allocs.
	RPC(method string, args interface{}, reply interface{}) error
}

// terminated is the interface needed by a prevAllocWatcher to check if an
// alloc is terminated.
type terminated interface {
	Terminated() bool
}

// AllocRunnerMeta provides metadata about an AllocRunner such as its alloc and
// alloc dir.
type AllocRunnerMeta interface {
	GetAllocDir() *allocdir.AllocDir
	Listener() *cstructs.AllocListener
	Alloc() *structs.Allocation
}

type Config struct {
	// Alloc is the current allocation which may need to block on its
	// previous allocation stopping.
	Alloc *structs.Allocation

	// PreviousRunner is non-nil if Alloc has a PreviousAllocation and it is
	// running locally.
	PreviousRunner AllocRunnerMeta

	// PreemptedRunners is non-nil if Alloc has one or more PreemptedAllocations.
	PreemptedRunners map[string]AllocRunnerMeta

	// RPC allows the alloc watcher to monitor remote allocations.
	RPC RPCer

	// Config is necessary for using the RPC.
	Config *config.Config

	// MigrateToken is used to migrate remote alloc dirs when ACLs are
	// enabled.
	MigrateToken string

	Logger hclog.Logger
}

func newMigratorForAlloc(c Config, tg *structs.TaskGroup, watchedAllocID string, m AllocRunnerMeta) config.PrevAllocMigrator {
	logger := c.Logger.Named("alloc_migrator").With("alloc_id", c.Alloc.ID).With("previous_alloc", watchedAllocID)

	tasks := tg.Tasks
	migrate := tg.EphemeralDisk != nil && tg.EphemeralDisk.Migrate
	sticky := tg.EphemeralDisk != nil && (tg.EphemeralDisk.Sticky || migrate)

	if m != nil {
		// Local Allocation because there's an alloc runner
		return &localPrevAlloc{
			allocID:      c.Alloc.ID,
			prevAllocID:  watchedAllocID,
			tasks:        tasks,
			sticky:       sticky,
			prevAllocDir: m.GetAllocDir(),
			prevListener: m.Listener(),
			prevStatus:   m.Alloc(),
			logger:       logger,
		}
	}

	return &remotePrevAlloc{
		allocID:      c.Alloc.ID,
		prevAllocID:  watchedAllocID,
		tasks:        tasks,
		config:       c.Config,
		migrate:      migrate,
		rpc:          c.RPC,
		migrateToken: c.MigrateToken,
		logger:       logger,
	}
}

// newWatcherForAlloc uses a local or rpc-based watcher depending on whether
// AllocRunnerMeta is nil or not.
//
// Note that c.Alloc.PreviousAllocation must NOT be used in this func as it
// used for preemption which has a distinct field. The caller is responsible
// for passing the allocation to be watched as watchedAllocID.
func newWatcherForAlloc(c Config, watchedAllocID string, m AllocRunnerMeta) config.PrevAllocWatcher {
	logger := c.Logger.Named("alloc_watcher").With("alloc_id", c.Alloc.ID).With("previous_alloc", watchedAllocID)

	if m != nil {
		// Local Allocation because there's an alloc runner
		return &localPrevAlloc{
			allocID:      c.Alloc.ID,
			prevAllocID:  watchedAllocID,
			prevAllocDir: m.GetAllocDir(),
			prevListener: m.Listener(),
			prevStatus:   m.Alloc(),
			logger:       logger,
		}
	}

	return &remotePrevAlloc{
		allocID:      c.Alloc.ID,
		prevAllocID:  watchedAllocID,
		config:       c.Config,
		rpc:          c.RPC,
		migrateToken: c.MigrateToken,
		logger:       logger,
	}
}

// NewAllocWatcher creates a PrevAllocWatcher if either PreviousAllocation or
// PreemptedRunners are set. If any of the allocs to watch have local runners,
// wait for them to terminate directly.
// For allocs which are either running on another node or have already
// terminated their alloc runners, use a remote backend which watches the alloc
// status via rpc.
func NewAllocWatcher(c Config) (config.PrevAllocWatcher, config.PrevAllocMigrator) {
	if c.Alloc.PreviousAllocation == "" && c.PreemptedRunners == nil {
		return NoopPrevAlloc{}, NoopPrevAlloc{}
	}

	var prevAllocWatchers []config.PrevAllocWatcher
	var prevAllocMigrator config.PrevAllocMigrator = NoopPrevAlloc{}

	// We have a previous allocation, add its listener to the watchers, and
	// use a migrator.
	if c.Alloc.PreviousAllocation != "" {
		tg := c.Alloc.Job.LookupTaskGroup(c.Alloc.TaskGroup)
		m := newMigratorForAlloc(c, tg, c.Alloc.PreviousAllocation, c.PreviousRunner)
		prevAllocWatchers = append(prevAllocWatchers, m)
		prevAllocMigrator = m
	}

	// We are preempting allocations, add their listeners to the watchers.
	if c.PreemptedRunners != nil {
		for aid, r := range c.PreemptedRunners {
			w := newWatcherForAlloc(c, aid, r)
			prevAllocWatchers = append(prevAllocWatchers, w)
		}
	}

	groupWatcher := &groupPrevAllocWatcher{
		prevAllocs: prevAllocWatchers,
	}

	return groupWatcher, prevAllocMigrator
}

// localPrevAlloc is a prevAllocWatcher for previous allocations on the same
// node as an updated allocation.
type localPrevAlloc struct {
	// allocID is the ID of the alloc being blocked
	allocID string

	// prevAllocID is the ID of the alloc being replaced
	prevAllocID string

	// tasks on the new alloc
	tasks []*structs.Task

	// sticky is true if data should be moved
	sticky bool

	// prevAllocDir is the alloc dir for the previous alloc
	prevAllocDir *allocdir.AllocDir

	// prevListener allows blocking for updates to the previous alloc
	prevListener *cstructs.AllocListener

	// prevStatus allows checking if the previous alloc has already
	// terminated (and therefore won't send updates to the listener)
	prevStatus terminated

	// waiting and migrating are true when alloc runner is waiting on the
	// prevAllocWatcher. Writers must acquire the waitingLock and readers
	// should use the helper methods IsWaiting and IsMigrating.
	waiting     bool
	migrating   bool
	waitingLock sync.RWMutex

	logger hclog.Logger
}

// IsWaiting returns true if there's a concurrent call inside Wait
func (p *localPrevAlloc) IsWaiting() bool {
	p.waitingLock.RLock()
	b := p.waiting
	p.waitingLock.RUnlock()
	return b
}

// IsMigrating returns true if there's a concurrent call inside Migrate
func (p *localPrevAlloc) IsMigrating() bool {
	p.waitingLock.RLock()
	b := p.migrating
	p.waitingLock.RUnlock()
	return b
}

// Wait on a local alloc to become terminal, exit, or the context to be done.
func (p *localPrevAlloc) Wait(ctx context.Context) error {
	p.waitingLock.Lock()
	p.waiting = true
	p.waitingLock.Unlock()
	defer func() {
		p.waitingLock.Lock()
		p.waiting = false
		p.waitingLock.Unlock()
	}()

	defer p.prevListener.Close()

	// Don't bother blocking for updates from the previous alloc if it has
	// already terminated.
	if p.prevStatus.Terminated() {
		p.logger.Trace("previous allocation already terminated")
		return nil
	}

	// Block until previous alloc exits
	p.logger.Debug("waiting for previous alloc to terminate")
	for {
		select {
		case prevAlloc, ok := <-p.prevListener.Ch():
			if !ok || prevAlloc.Terminated() {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Migrate from previous local alloc dir to destination alloc dir.
func (p *localPrevAlloc) Migrate(ctx context.Context, dest *allocdir.AllocDir) error {
	if !p.sticky {
		// Not a sticky volume, nothing to migrate
		return nil
	}

	p.waitingLock.Lock()
	p.migrating = true
	p.waitingLock.Unlock()
	defer func() {
		p.waitingLock.Lock()
		p.migrating = false
		p.waitingLock.Unlock()
	}()

	p.logger.Debug("copying previous alloc")

	return dest.Move(p.prevAllocDir, p.tasks)
}

// remotePrevAlloc is a prevAllocWatcher for previous allocations on remote
// nodes as an updated allocation.
type remotePrevAlloc struct {
	// allocID is the ID of the alloc being blocked
	allocID string

	// prevAllocID is the ID of the alloc being replaced
	prevAllocID string

	// tasks on the new alloc
	tasks []*structs.Task

	// config for the Client to get AllocDir, Region, and Node.SecretID
	config *config.Config

	// migrate is true if data should be moved between nodes
	migrate bool

	// rpc provides an RPC method for watching for updates to the previous
	// alloc and determining what node it was on.
	rpc RPCer

	// nodeID is the node the previous alloc. Set by Wait() for use in
	// Migrate() iff the previous alloc has not already been GC'd.
	nodeID string

	// waiting and migrating are true when alloc runner is waiting on the
	// prevAllocWatcher. Writers must acquire the waitingLock and readers
	// should use the helper methods IsWaiting and IsMigrating.
	waiting     bool
	migrating   bool
	waitingLock sync.RWMutex

	logger hclog.Logger

	// migrateToken allows a client to migrate data in an ACL-protected remote
	// volume
	migrateToken string
}

// IsWaiting returns true if there's a concurrent call inside Wait
func (p *remotePrevAlloc) IsWaiting() bool {
	p.waitingLock.RLock()
	b := p.waiting
	p.waitingLock.RUnlock()
	return b
}

// IsMigrating returns true if there's a concurrent call inside Migrate
func (p *remotePrevAlloc) IsMigrating() bool {
	p.waitingLock.RLock()
	b := p.migrating
	p.waitingLock.RUnlock()
	return b
}

// Wait until the remote previous allocation has terminated.
func (p *remotePrevAlloc) Wait(ctx context.Context) error {
	p.waitingLock.Lock()
	p.waiting = true
	p.waitingLock.Unlock()
	defer func() {
		p.waitingLock.Lock()
		p.waiting = false
		p.waitingLock.Unlock()
	}()

	p.logger.Debug("waiting for remote previous alloc to terminate")
	req := structs.AllocSpecificRequest{
		AllocID: p.prevAllocID,
		QueryOptions: structs.QueryOptions{
			Region:     p.config.Region,
			AllowStale: true,
			AuthToken:  p.config.Node.SecretID,
		},
	}

	done := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	for !done() {
		resp := structs.SingleAllocResponse{}
		err := p.rpc.RPC("Alloc.GetAlloc", &req, &resp)
		if err != nil {
			p.logger.Error("error querying previous alloc", "error", err)
			retry := getRemoteRetryIntv + helper.RandomStagger(getRemoteRetryIntv)
			select {
			case <-time.After(retry):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if resp.Alloc == nil {
			p.logger.Debug("blocking alloc was GC'd")
			return nil
		}
		if resp.Alloc.Terminated() || resp.Alloc.ClientStatus == structs.AllocClientStatusUnknown {
			p.nodeID = resp.Alloc.NodeID
			return nil
		}

		// Update the query index and requery.
		if resp.Index > req.MinQueryIndex {
			req.MinQueryIndex = resp.Index
		}
	}

	return ctx.Err()
}

// Migrate alloc data from a remote node if the new alloc has migration enabled
// and the old alloc hasn't been GC'd.
func (p *remotePrevAlloc) Migrate(ctx context.Context, dest *allocdir.AllocDir) error {
	if !p.migrate {
		// Volume wasn't configured to be migrated, return early
		return nil
	}

	p.waitingLock.Lock()
	p.migrating = true
	p.waitingLock.Unlock()
	defer func() {
		p.waitingLock.Lock()
		p.migrating = false
		p.waitingLock.Unlock()
	}()

	p.logger.Debug("copying from remote previous alloc")

	if p.nodeID == "" {
		// NodeID couldn't be found; likely alloc was GC'd
		p.logger.Warn("unable to migrate data from previous alloc; previous alloc may have been GC'd")
		return nil
	}

	addr, err := p.getNodeAddr(ctx, p.nodeID)
	if err != nil {
		return err
	}

	prevAllocDir, err := p.migrateAllocDir(ctx, addr)
	if err != nil {
		return err
	}

	if err := dest.Move(prevAllocDir, p.tasks); err != nil {
		// cleanup on error
		prevAllocDir.Destroy()
		return err
	}

	if err := prevAllocDir.Destroy(); err != nil {
		p.logger.Error("error destroying alloc dir",
			"error", err, "previous_alloc_dir", prevAllocDir.AllocDir)
	}
	return nil
}

// getNodeAddr gets the node from the server with the given Node ID
func (p *remotePrevAlloc) getNodeAddr(ctx context.Context, nodeID string) (string, error) {
	req := structs.NodeSpecificRequest{
		NodeID: nodeID,
		QueryOptions: structs.QueryOptions{
			Region:     p.config.Region,
			AllowStale: true,
			AuthToken:  p.config.Node.SecretID,
		},
	}

	resp := structs.SingleNodeResponse{}
	for {
		err := p.rpc.RPC("Node.GetNode", &req, &resp)
		if err != nil {
			p.logger.Error("failed to query node", "error", err, "node", nodeID)
			retry := getRemoteRetryIntv + helper.RandomStagger(getRemoteRetryIntv)
			select {
			case <-time.After(retry):
				continue
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		break
	}

	if resp.Node == nil {
		return "", fmt.Errorf("node %q not found", nodeID)
	}

	scheme := "http://"
	if resp.Node.TLSEnabled {
		scheme = "https://"
	}
	return scheme + resp.Node.HTTPAddr, nil
}

// migrate a remote alloc dir to local node. Caller is responsible for calling
// Destroy on the returned allocdir if no error occurs.
func (p *remotePrevAlloc) migrateAllocDir(ctx context.Context, nodeAddr string) (*allocdir.AllocDir, error) {
	// Create the previous alloc dir
	prevAllocDir := allocdir.NewAllocDir(p.logger, p.config.AllocDir, p.prevAllocID)
	if err := prevAllocDir.Build(); err != nil {
		return nil, fmt.Errorf("error building alloc dir for previous alloc %q: %v", p.prevAllocID, err)
	}

	// Create an API client
	apiConfig := nomadapi.DefaultConfig()
	apiConfig.Address = nodeAddr
	apiConfig.TLSConfig = &nomadapi.TLSConfig{
		CACert:        p.config.TLSConfig.CAFile,
		ClientCert:    p.config.TLSConfig.CertFile,
		ClientKey:     p.config.TLSConfig.KeyFile,
		TLSServerName: fmt.Sprintf("client.%s.nomad", p.config.Region),
	}
	apiClient, err := nomadapi.NewClient(apiConfig)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("/v1/client/allocation/%v/snapshot", p.prevAllocID)
	qo := &nomadapi.QueryOptions{AuthToken: p.migrateToken}
	resp, err := apiClient.Raw().Response(url, qo)
	if err != nil {
		prevAllocDir.Destroy()
		return nil, fmt.Errorf("error getting snapshot from previous alloc %q: %v", p.prevAllocID, err)
	}

	if err := p.streamAllocDir(ctx, resp, prevAllocDir.AllocDir); err != nil {
		prevAllocDir.Destroy()
		return nil, err
	}

	return prevAllocDir, nil
}

// stream remote alloc to dir to a local path. Caller should cleanup dest on
// error.
func (p *remotePrevAlloc) streamAllocDir(ctx context.Context, resp io.ReadCloser, dest string) error {
	p.logger.Debug("streaming snapshot of previous alloc", "destination", dest)
	tr := tar.NewReader(resp)
	defer resp.Close()

	// Cache effective uid as we only run Chown if we're root
	euid := syscall.Geteuid()

	canceled := func() bool {
		select {
		case <-ctx.Done():
			p.logger.Info("migration of previous alloc canceled")
			return true
		default:
			return false
		}
	}

	// if we see this file, there was an error on the remote side
	errorFilename := allocdir.SnapshotErrorFilename(p.prevAllocID)

	buf := make([]byte, 1024)
	for !canceled() {
		// Get the next header
		hdr, err := tr.Next()

		// Snapshot has ended
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return fmt.Errorf("error streaming previous alloc %q for new alloc %q: %v",
				p.prevAllocID, p.allocID, err)
		}

		if hdr.Name == errorFilename {
			// Error snapshotting on the remote side, try to read
			// the message out of the file and return it.
			errBuf := make([]byte, int(hdr.Size))
			if _, err := tr.Read(errBuf); err != nil && err != io.EOF {
				return fmt.Errorf("error streaming previous alloc %q for new alloc %q; failed reading error message: %v",
					p.prevAllocID, p.allocID, err)
			}
			return fmt.Errorf("error streaming previous alloc %q for new alloc %q: %s",
				p.prevAllocID, p.allocID, string(errBuf))
		}

		// If the header is for a directory we create the directory
		if hdr.Typeflag == tar.TypeDir {
			name := filepath.Join(dest, hdr.Name)
			os.MkdirAll(name, os.FileMode(hdr.Mode))

			// Can't change owner if not root or on Windows.
			if euid == 0 {
				if err := os.Chown(name, hdr.Uid, hdr.Gid); err != nil {
					return fmt.Errorf("error chowning directory %v", err)
				}
			}
			continue
		}
		// If the header is for a symlink we create the symlink
		if hdr.Typeflag == tar.TypeSymlink {
			if err = os.Symlink(hdr.Linkname, filepath.Join(dest, hdr.Name)); err != nil {
				return fmt.Errorf("error creating symlink: %v", err)
			}
			continue
		}
		// If the header is a file, we write to a file
		if hdr.Typeflag == tar.TypeReg {
			f, err := os.Create(filepath.Join(dest, hdr.Name))
			if err != nil {
				return fmt.Errorf("error creating file: %v", err)
			}

			// Setting the permissions of the file as the origin.
			if err := f.Chmod(os.FileMode(hdr.Mode)); err != nil {
				f.Close()
				return fmt.Errorf("error chmoding file %v", err)
			}

			// Can't change owner if not root or on Windows.
			if euid == 0 {
				if err := f.Chown(hdr.Uid, hdr.Gid); err != nil {
					f.Close()
					return fmt.Errorf("error chowning file %v", err)
				}
			}

			// We write in chunks so that we can test if the client
			// is still alive
			for !canceled() {
				n, err := tr.Read(buf)
				if n > 0 && (err == nil || err == io.EOF) {
					if _, err := f.Write(buf[:n]); err != nil {
						f.Close()
						return fmt.Errorf("error writing to file %q: %v", f.Name(), err)
					}
				}

				if err != nil {
					f.Close()
					if err != io.EOF {
						return fmt.Errorf("error reading snapshot: %v", err)
					}
					break
				}
			}

		}
	}

	if canceled() {
		return ctx.Err()
	}

	return nil
}

// NoopPrevAlloc does not block or migrate on a previous allocation and never
// returns an error.
type NoopPrevAlloc struct{}

// Wait returns nil immediately.
func (NoopPrevAlloc) Wait(context.Context) error { return nil }

// Migrate returns nil immediately.
func (NoopPrevAlloc) Migrate(context.Context, *allocdir.AllocDir) error { return nil }

func (NoopPrevAlloc) IsWaiting() bool   { return false }
func (NoopPrevAlloc) IsMigrating() bool { return false }
