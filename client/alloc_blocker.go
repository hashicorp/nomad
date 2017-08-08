package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocGetter is able to retrieve local and remote allocs.
type allocGetter interface {
	// GetClientAlloc returns the alloc if an alloc ID is found locally,
	// otherwise an error.
	GetClientAlloc(allocID string) (*structs.Allocation, error)

	// RPC allows retrieving remote allocs.
	RPC(method string, args interface{}, reply interface{}) error
}

type allocBlocker struct {
	// blocking is a map of allocs being watched to chans to signal their
	// termination and optionally the node they were running on.
	blocking     map[string]chan string
	blockingLock sync.Mutex

	// allocs is used to retrieve local and remote allocs
	allocs allocGetter

	// region for making rpc calls
	region string

	logger *log.Logger
}

func newAllocBlocker(l *log.Logger, allocs allocGetter, region string) *allocBlocker {
	return &allocBlocker{
		blocking: make(map[string]chan string),
		allocs:   allocs,
		region:   region,
		logger:   l,
	}
}

// allocTerminated marks a local allocation as terminated or GC'd.
func (a *allocBlocker) allocTerminated(allocID string) {
	a.blockingLock.Lock()
	defer a.blockingLock.Unlock()
	if ch, ok := a.blocking[allocID]; ok {
		//TODO(schmichael) REMOVE
		a.logger.Printf("[TRACE] client: XXX closing and deleting terminated blocking alloc %q", allocID)
		ch <- ""
		delete(a.blocking, allocID)
	} else {
		//TODO(schmichael) REMOVE
		a.logger.Printf("[TRACE] client: XXX no waiting on terminated alloc %q", allocID)
	}
}

// BlockOnAlloc blocks on an alloc terminating.
func (a *allocBlocker) BlockOnAlloc(ctx context.Context, allocID string) (string, error) {
	// Register an intent to block until an alloc exists to prevent races
	// between checking to see if it has already exited and waiting for it
	// to exit
	terminatedCh, err := a.watch(allocID)
	if err != nil {
		return "", err
	}

	if alloc, err := a.allocs.GetClientAlloc(allocID); err == nil {
		// Local alloc, return early if already terminated
		if alloc.Terminated() {
			return "", nil
		}
	} else {
		// Remote alloc, setup blocking rpc call
		go a.watchRemote(ctx, allocID)
	}

	select {
	case node := <-terminatedCh:
		a.logger.Printf("[DEBUG] client: blocking alloc %q exited", allocID)
		//TODO migrate?!
		return node, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// watch for an alloc to terminate. Returns an error if there's already a
// watcher as blocked allocs to blockers should be 1:1.
func (a *allocBlocker) watch(allocID string) (<-chan string, error) {
	a.blockingLock.Lock()
	defer a.blockingLock.Unlock()

	ch, ok := a.blocking[allocID]
	if ok {
		return nil, fmt.Errorf("multiple blockers on alloc %q", allocID)
	}

	ch = make(chan string)
	a.blocking[allocID] = ch
	return ch
}

// watch for a non-local alloc to terminate using a blocking rpc call
func (a *allocBlocker) watchRemote(ctx context.Context, allocID string) {
	req := structs.AllocSpecificRequest{
		AllocID: allocID,
		QueryOptions: structs.QueryOptions{
			Region:     a.region,
			AllowStale: true,
		},
	}

	for {
		resp := structs.SingleAllocResponse{}
		err := a.allocs.RPC("Alloc.GetAlloc", &req, &resp)
		if err != nil {
			a.logger.Printf("[ERR] client: failed to query allocation %q: %v", allocID, err)
			retry := getAllocRetryIntv + lib.RandomStagger(getAllocRetryIntv)
			select {
			case <-time.After(retry):
				continue
			case <-ctx.Done():
				return
			}
		}
		if resp.Alloc == nil {
			//TODO(schmichael) confirm this assumption
			a.logger.Printf("[DEBUG] client: blocking alloc %q has been GC'd", allocID)
			a.allocTerminated(allocID, "")
		}
		if resp.Alloc.Terminated() {
			// Terminated!
			a.allocTerminated(allocID, resp.Alloc.NodeID)
		}

		// Update the query index and requery.
		if resp.Index > req.MinQueryIndex {
			req.MinQueryIndex = resp.Index
		}
	}

}

// GetNodeAddr gets the node from the server with the given Node ID
func (a *allocBlocker) GetNodeAddr(ctx context.Context, nodeID string) (*structs.Node, error) {
	req := structs.NodeSpecificRequest{
		NodeID: nodeID,
		QueryOptions: structs.QueryOptions{
			Region:     c.region,
			AllowStale: true,
		},
	}

	resp := structs.SingleNodeResponse{}
	for {
		err := c.allocs.RPC("Node.GetNode", &req, &resp)
		if err != nil {
			c.logger.Printf("[ERR] client: failed to query node info %q: %v", nodeID, err)
			retry := getAllocRetryIntv + lib.RandomStagger(getAllocRetryIntv)
			select {
			case <-time.After(retry):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		break
	}

	if resp.Node == nil {
		return nil, fmt.Errorf("node %q not found", nodeID)
	}

	scheme := "http://"
	if node.TLSEnabled {
		scheme = "https://"
	}
	return scheme + node.HTTPAdrr, nil
}
