// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultclient

import (
	"container/heap"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vaultapi "github.com/hashicorp/vault/api"
)

// TokenDeriverFunc takes in an allocation and a set of tasks and derives a
// wrapped token for all the tasks, from the nomad server. All the derived
// wrapped tokens will be unwrapped using the vault API client.
type TokenDeriverFunc func(*structs.Allocation, []string, *vaultapi.Client) (map[string]string, error)

// VaultClient is the interface which nomad client uses to interact with vault and
// periodically renews the tokens and secrets.
type VaultClient interface {
	// Start initiates the renewal loop of tokens and secrets
	Start()

	// Stop terminates the renewal loop for tokens and secrets
	Stop()

	// DeriveToken contacts the nomad server and fetches wrapped tokens for
	// a set of tasks. The wrapped tokens will be unwrapped using vault and
	// returned.
	DeriveToken(*structs.Allocation, []string) (map[string]string, error)

	// GetConsulACL fetches the Consul ACL token required for the task
	GetConsulACL(string, string) (*vaultapi.Secret, error)

	// RenewToken renews a token with the given increment and adds it to
	// the min-heap for periodic renewal.
	RenewToken(string, int) (<-chan error, error)

	// StopRenewToken removes the token from the min-heap, stopping its
	// renewal.
	StopRenewToken(string) error
}

// Implementation of VaultClient interface to interact with vault and perform
// token and lease renewals periodically.
type vaultClient struct {
	// tokenDeriver is a function pointer passed in by the client to derive
	// tokens by making RPC calls to the nomad server. The wrapped tokens
	// returned by the nomad server will be unwrapped by this function
	// using the vault API client.
	tokenDeriver TokenDeriverFunc

	// running indicates if the renewal loop is active or not
	running bool

	// client is the API client to interact with vault
	client *vaultapi.Client

	// updateCh is the channel to notify heap modifications to the renewal
	// loop
	updateCh chan struct{}

	// stopCh is the channel to trigger termination of renewal loop
	stopCh chan struct{}

	// heap is the min-heap to keep track of both tokens and leases
	heap *vaultClientHeap

	// config is the configuration to connect to vault
	config *config.VaultConfig

	lock   sync.RWMutex
	logger hclog.Logger
}

// vaultClientRenewalRequest is a request object for renewal of both tokens and
// secret's leases.
type vaultClientRenewalRequest struct {
	// errCh is the channel into which any renewal error will be sent to
	errCh chan error

	// id is an identifier which represents either a token or a lease
	id string

	// increment is the duration for which the token or lease should be
	// renewed for
	increment int

	// isToken indicates whether the 'id' field is a token or not
	isToken bool
}

// Element representing an entry in the renewal heap
type vaultClientHeapEntry struct {
	req   *vaultClientRenewalRequest
	next  time.Time
	index int
}

// Wrapper around the actual heap to provide additional semantics on top of
// functions provided by the heap interface. In order to achieve that, an
// additional map is placed beside the actual heap. This map can be used to
// check if an entry is already present in the heap.
type vaultClientHeap struct {
	heapMap map[string]*vaultClientHeapEntry
	heap    vaultDataHeapImp
}

// Data type of the heap
type vaultDataHeapImp []*vaultClientHeapEntry

// NewVaultClient returns a new vault client from the given config.
func NewVaultClient(config *config.VaultConfig, logger hclog.Logger, tokenDeriver TokenDeriverFunc) (*vaultClient, error) {
	if config == nil {
		return nil, fmt.Errorf("nil vault config")
	}

	logger = logger.Named("vault")

	c := &vaultClient{
		config: config,
		stopCh: make(chan struct{}),
		// Update channel should be a buffered channel
		updateCh:     make(chan struct{}, 1),
		heap:         newVaultClientHeap(),
		logger:       logger,
		tokenDeriver: tokenDeriver,
	}

	if !config.IsEnabled() {
		return c, nil
	}

	// Get the Vault API configuration
	apiConf, err := config.ApiConfig()
	if err != nil {
		logger.Error("error creating vault API config", "error", err)
		return nil, err
	}

	// Create the Vault API client
	client, err := vaultapi.NewClient(apiConf)
	if err != nil {
		logger.Error("error creating vault client", "error", err)
		return nil, err
	}

	// Set our Nomad user agent
	useragent.SetHeaders(client)

	// SetHeaders above will replace all headers, make this call second
	if config.Namespace != "" {
		logger.Debug("configuring Vault namespace", "namespace", config.Namespace)
		client.SetNamespace(config.Namespace)
	}

	c.client = client

	return c, nil
}

// newVaultClientHeap returns a new vault client heap with both the heap and a
// map which is a secondary index for heap elements, both initialized.
func newVaultClientHeap() *vaultClientHeap {
	return &vaultClientHeap{
		heapMap: make(map[string]*vaultClientHeapEntry),
		heap:    make(vaultDataHeapImp, 0),
	}
}

// isTracked returns if a given identifier is already present in the heap and
// hence is being renewed. Lock should be held before calling this method.
func (c *vaultClient) isTracked(id string) bool {
	if id == "" {
		return false
	}

	_, ok := c.heap.heapMap[id]
	return ok
}

// isRunning returns true if the client is running.
func (c *vaultClient) isRunning() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.running
}

// Start starts the renewal loop of vault client
func (c *vaultClient) Start() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.config.IsEnabled() || c.running {
		return
	}

	c.running = true

	go c.run()
}

// Stop stops the renewal loop of vault client
func (c *vaultClient) Stop() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.config.IsEnabled() || !c.running {
		return
	}

	c.running = false
	close(c.stopCh)
}

// unlockAndUnset is used to unset the vault token on the client and release the
// lock. Helper method for deferring a call that does both.
func (c *vaultClient) unlockAndUnset() {
	c.client.SetToken("")
	c.lock.Unlock()
}

// DeriveToken takes in an allocation and a set of tasks and for each of the
// task, it derives a vault token from nomad server and unwraps it using vault.
// The return value is a map containing all the unwrapped tokens indexed by the
// task name.
func (c *vaultClient) DeriveToken(alloc *structs.Allocation, taskNames []string) (map[string]string, error) {
	if !c.config.IsEnabled() {
		return nil, fmt.Errorf("vault client not enabled")
	}
	if !c.isRunning() {
		return nil, fmt.Errorf("vault client is not running")
	}

	c.lock.Lock()
	defer c.unlockAndUnset()

	// Use the token supplied to interact with vault
	c.client.SetToken("")

	tokens, err := c.tokenDeriver(alloc, taskNames, c.client)
	if err != nil {
		c.logger.Error("error deriving token", "error", err, "alloc_id", alloc.ID, "task_names", taskNames)
		return nil, err
	}

	return tokens, nil
}

// GetConsulACL creates a vault API client and reads from vault a consul ACL
// token used by the task.
func (c *vaultClient) GetConsulACL(token, path string) (*vaultapi.Secret, error) {
	if !c.config.IsEnabled() {
		return nil, fmt.Errorf("vault client not enabled")
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if path == "" {
		return nil, fmt.Errorf("missing consul ACL token vault path")
	}

	c.lock.Lock()
	defer c.unlockAndUnset()

	// Use the token supplied to interact with vault
	c.client.SetToken(token)

	// Read the consul ACL token and return the secret directly
	return c.client.Logical().Read(path)
}

// RenewToken renews the supplied token for a given duration (in seconds) and
// adds it to the min-heap so that it is renewed periodically by the renewal
// loop. Any error returned during renewal will be written to a buffered
// channel and the channel is returned instead of an actual error. This helps
// the caller be notified of a renewal failure asynchronously for appropriate
// actions to be taken. The caller of this function need not have to close the
// error channel.
func (c *vaultClient) RenewToken(token string, increment int) (<-chan error, error) {
	if token == "" {
		err := fmt.Errorf("missing token")
		return nil, err
	}
	if increment < 1 {
		err := fmt.Errorf("increment cannot be less than 1")
		return nil, err
	}

	// Create a buffered error channel
	errCh := make(chan error, 1)

	// Create a renewal request and indicate that the identifier in the
	// request is a token and not a lease
	renewalReq := &vaultClientRenewalRequest{
		errCh:     errCh,
		id:        token,
		isToken:   true,
		increment: increment,
	}

	// Perform the renewal of the token and send any error to the dedicated
	// error channel.
	if err := c.renew(renewalReq); err != nil {
		c.logger.Error("error during renewal of token", "error", err)
		metrics.IncrCounter([]string{"client", "vault", "renew_token_failure"}, 1)
		return nil, err
	}

	return errCh, nil
}

// renew is a common method to handle renewal of both tokens and secret leases.
// It invokes a token renewal or a secret's lease renewal. If renewal is
// successful, min-heap is updated based on the duration after which it needs
// renewal again. The next renewal time is randomly selected to avoid spikes in
// the number of APIs periodically.
func (c *vaultClient) renew(req *vaultClientRenewalRequest) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if req == nil {
		return fmt.Errorf("nil renewal request")
	}
	if req.errCh == nil {
		return fmt.Errorf("renewal request error channel nil")
	}

	if !c.config.IsEnabled() {
		close(req.errCh)
		return fmt.Errorf("vault client not enabled")
	}
	if !c.running {
		close(req.errCh)
		return fmt.Errorf("vault client is not running")
	}
	if req.id == "" {
		close(req.errCh)
		return fmt.Errorf("missing id in renewal request")
	}
	if req.increment < 1 {
		close(req.errCh)
		return fmt.Errorf("increment cannot be less than 1")
	}

	var renewalErr error
	leaseDuration := req.increment
	if req.isToken {
		// Set the token in the API client to the one that needs renewal
		c.client.SetToken(req.id)

		// Renew the token
		renewResp, err := c.client.Auth().Token().RenewSelf(req.increment)
		if err != nil {
			renewalErr = fmt.Errorf("failed to renew the vault token: %v", err)
		} else if renewResp == nil || renewResp.Auth == nil {
			renewalErr = fmt.Errorf("failed to renew the vault token")
		} else {
			// Don't set this if renewal fails
			leaseDuration = renewResp.Auth.LeaseDuration
		}

		// Reset the token in the API client before returning
		c.client.SetToken("")
	} else {
		// Renew the secret
		renewResp, err := c.client.Sys().Renew(req.id, req.increment)
		if err != nil {
			renewalErr = fmt.Errorf("failed to renew vault secret: %v", err)
		} else if renewResp == nil {
			renewalErr = fmt.Errorf("failed to renew vault secret")
		} else {
			// Don't set this if renewal fails
			leaseDuration = renewResp.LeaseDuration
		}
	}

	// Determine the next renewal time
	renewalDuration := renewalTime(rand.Intn, leaseDuration)
	next := time.Now().Add(renewalDuration)

	fatal := false
	if renewalErr != nil &&
		(strings.Contains(renewalErr.Error(), "lease not found or lease is not renewable") ||
			strings.Contains(renewalErr.Error(), "lease is not renewable") ||
			strings.Contains(renewalErr.Error(), "token not found") ||
			strings.Contains(renewalErr.Error(), "permission denied")) {
		fatal = true
	} else if renewalErr != nil {
		c.logger.Debug("renewal error details", "req.increment", req.increment, "lease_duration", leaseDuration, "renewal_duration", renewalDuration)
		c.logger.Error("error during renewal of lease or token failed due to a non-fatal error; retrying",
			"error", renewalErr, "period", next)
	}

	if c.isTracked(req.id) {
		if fatal {
			// If encountered with an error where in a lease or a
			// token is not valid at all with vault, and if that
			// item is tracked by the renewal loop, stop renewing
			// it by removing the corresponding heap entry.
			if err := c.heap.Remove(req.id); err != nil {
				return fmt.Errorf("failed to remove heap entry: %v", err)
			}

			// Report the fatal error to the client
			req.errCh <- renewalErr
			close(req.errCh)

			return renewalErr
		}

		// If the identifier is already tracked, this indicates a
		// subsequest renewal. In this case, update the existing
		// element in the heap with the new renewal time.
		if err := c.heap.Update(req, next); err != nil {
			return fmt.Errorf("failed to update heap entry. err: %v", err)
		}

		// There is no need to signal an update to the renewal loop
		// here because this case is hit from the renewal loop itself.
	} else {
		if fatal {
			// If encountered with an error where in a lease or a
			// token is not valid at all with vault, and if that
			// item is not tracked by renewal loop, don't add it.

			// Report the fatal error to the client
			req.errCh <- renewalErr
			close(req.errCh)

			return renewalErr
		}

		// If the identifier is not already tracked, this is a first
		// renewal request. In this case, add an entry into the heap
		// with the next renewal time.
		if err := c.heap.Push(req, next); err != nil {
			return fmt.Errorf("failed to push an entry to heap.  err: %v", err)
		}

		// Signal an update for the renewal loop to trigger a fresh
		// computation for the next best candidate for renewal.
		if c.running {
			select {
			case c.updateCh <- struct{}{}:
			default:
			}
		}
	}

	return nil
}

// run is the renewal loop which performs the periodic renewals of both the
// tokens and the secret leases.
func (c *vaultClient) run() {
	if !c.config.IsEnabled() {
		return
	}

	var renewalCh <-chan time.Time
	for c.config.IsEnabled() && c.isRunning() {
		// Fetches the candidate for next renewal
		renewalReq, renewalTime := c.nextRenewal()
		if renewalTime.IsZero() {
			// If the heap is empty, don't do anything
			renewalCh = nil
		} else {
			now := time.Now()
			if renewalTime.After(now) {
				// Compute the duration after which the item
				// needs renewal and set the renewalCh to fire
				// at that time.
				renewalDuration := time.Until(renewalTime)
				renewalCh = time.After(renewalDuration)
			} else {
				// If the renewals of multiple items are too
				// close to each other and by the time the
				// entry is fetched from heap it might be past
				// the current time (by a small margin). In
				// which case, fire immediately.
				renewalCh = time.After(0)
			}
		}

		select {
		case <-renewalCh:
			if err := c.renew(renewalReq); err != nil {
				c.logger.Error("error renewing token", "error", err)
				metrics.IncrCounter([]string{"client", "vault", "renew_token_error"}, 1)
			}
		case <-c.updateCh:
			continue
		case <-c.stopCh:
			c.logger.Debug("stopped")
			return
		}
	}
}

// StopRenewToken removes the item from the heap which represents the given
// token.
func (c *vaultClient) StopRenewToken(token string) error {
	return c.stopRenew(token)
}

// stopRenew removes the given identifier from the heap and signals the renewal
// loop to compute the next best candidate for renewal.
func (c *vaultClient) stopRenew(id string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.isTracked(id) {
		return nil
	}

	if err := c.heap.Remove(id); err != nil {
		return fmt.Errorf("failed to remove heap entry: %v", err)
	}

	// Signal an update to the renewal loop.
	if c.running {
		select {
		case c.updateCh <- struct{}{}:
		default:
		}
	}

	return nil
}

// nextRenewal returns the root element of the min-heap, which represents the
// next element to be renewed and the time at which the renewal needs to be
// triggered.
func (c *vaultClient) nextRenewal() (*vaultClientRenewalRequest, time.Time) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.heap.Length() == 0 {
		return nil, time.Time{}
	}

	// Fetches the root element in the min-heap
	nextEntry := c.heap.Peek()
	if nextEntry == nil {
		return nil, time.Time{}
	}

	return nextEntry.req, nextEntry.next
}

// Additional helper functions on top of interface methods

// Length returns the number of elements in the heap
func (h *vaultClientHeap) Length() int {
	return len(h.heap)
}

// Returns the root node of the min-heap
func (h *vaultClientHeap) Peek() *vaultClientHeapEntry {
	if len(h.heap) == 0 {
		return nil
	}

	return h.heap[0]
}

// Push adds the secondary index and inserts an item into the heap
func (h *vaultClientHeap) Push(req *vaultClientRenewalRequest, next time.Time) error {
	if req == nil {
		return fmt.Errorf("nil request")
	}

	if _, ok := h.heapMap[req.id]; ok {
		return fmt.Errorf("entry %v already exists", req.id)
	}

	heapEntry := &vaultClientHeapEntry{
		req:  req,
		next: next,
	}
	h.heapMap[req.id] = heapEntry
	heap.Push(&h.heap, heapEntry)
	return nil
}

// Update will modify the existing item in the heap with the new data and the
// time, and fixes the heap.
func (h *vaultClientHeap) Update(req *vaultClientRenewalRequest, next time.Time) error {
	if entry, ok := h.heapMap[req.id]; ok {
		entry.req = req
		entry.next = next
		heap.Fix(&h.heap, entry.index)
		return nil
	}

	return fmt.Errorf("heap doesn't contain %v", req.id)
}

// Remove will remove an identifier from the secondary index and deletes the
// corresponding node from the heap.
func (h *vaultClientHeap) Remove(id string) error {
	if entry, ok := h.heapMap[id]; ok {
		heap.Remove(&h.heap, entry.index)
		delete(h.heapMap, id)
		return nil
	}

	return fmt.Errorf("heap doesn't contain entry for %v", id)
}

// The heap interface requires the following methods to be implemented.
// * Push(x interface{}) // add x as element Len()
// * Pop() interface{}   // remove and return element Len() - 1.
// * sort.Interface
//
// sort.Interface comprises of the following methods:
// * Len() int
// * Less(i, j int) bool
// * Swap(i, j int)

// Part of sort.Interface
func (h vaultDataHeapImp) Len() int { return len(h) }

// Part of sort.Interface
func (h vaultDataHeapImp) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	// Sort such that zero times are at the end of the list.
	iZero, jZero := h[i].next.IsZero(), h[j].next.IsZero()
	if iZero && jZero {
		return false
	} else if iZero {
		return false
	} else if jZero {
		return true
	}

	return h[i].next.Before(h[j].next)
}

// Part of sort.Interface
func (h vaultDataHeapImp) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

// Part of heap.Interface
func (h *vaultDataHeapImp) Push(x interface{}) {
	n := len(*h)
	entry := x.(*vaultClientHeapEntry)
	entry.index = n
	*h = append(*h, entry)
}

// Part of heap.Interface
func (h *vaultDataHeapImp) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	entry.index = -1 // for safety
	*h = old[0 : n-1]
	return entry
}

// randIntn is the function in math/rand needed by renewalTime. A type is used
// to ease deterministic testing.
type randIntn func(int) int

// renewalTime returns when a token should be renewed given its leaseDuration
// and a randomizer to provide jitter.
//
// Leases < 1m will be not jitter.
func renewalTime(dice randIntn, leaseDuration int) time.Duration {
	// Start trying to renew at half the lease duration to allow ample time
	// for latency and retries.
	renew := leaseDuration / 2

	// Don't bother about introducing randomness if the
	// leaseDuration is too small.
	const cutoff = 30
	if renew < cutoff {
		return time.Duration(renew) * time.Second
	}

	// jitter is the amount +/- to vary the renewal time
	const jitter = 10
	min := renew - jitter
	renew = min + dice(jitter*2)

	return time.Duration(renew) * time.Second
}
