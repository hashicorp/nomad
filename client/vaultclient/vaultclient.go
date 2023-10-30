// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultclient

import (
	"container/heap"
	"context"
	"errors"
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

// VaultClientFunc is the interface of a function that retreives the VaultClient
// by cluster name. This function is injected into the allocrunner/taskrunner
type VaultClientFunc func(string) (VaultClient, error)

// TokenDeriverFunc takes in an allocation and a set of tasks and derives a
// wrapped token for all the tasks, from the nomad server. All the derived
// wrapped tokens will be unwrapped using the vault API client.
type TokenDeriverFunc func(*structs.Allocation, []string, *vaultapi.Client) (map[string]string, error)

// JWTLoginRequest is used to derive a Vault ACL token using a JWT login
// request.
type JWTLoginRequest struct {
	// JWT is the signed JWT to be used for the login request.
	JWT string

	// Role is Vault ACL role to use for the login request. If empty, the
	// Nomad client's create_from_role value is used, or the Vault cluster
	// default role.
	Role string
}

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

	// DeriveTokenWithJWT returns a Vault ACL token using the JWT login
	// endpoint.
	DeriveTokenWithJWT(context.Context, JWTLoginRequest) (string, error)

	// GetConsulACL fetches the Consul ACL token required for the task
	GetConsulACL(string, string) (*vaultapi.Secret, error)

	// RenewToken renews a token with the given increment and adds it to
	// the min-heap for periodic renewal.
	RenewToken(string, int) (<-chan error, error)

	// StopRenewToken removes the token from the min-heap, stopping its
	// renewal.
	StopRenewToken(string) error
}

// Client is the implementation of VaultClient interface to interact with vault
// and perform token and lease renewals periodically.
type Client struct {
	// TokenDeriver is a function pointer passed in by the Vault to derive
	// tokens by making RPC calls to the nomad server. The wrapped tokens
	// returned by the nomad server will be unwrapped by this function
	// using the vault API client.
	TokenDeriver TokenDeriverFunc

	// Running indicates if the renewal loop is active or not
	Running bool

	// Vault is the API Client to interact with vault
	Vault *vaultapi.Client

	// UpdateCh is the channel to notify Heap modifications to the renewal
	// loop
	UpdateCh chan struct{}

	// StopCh is the channel to trigger termination of renewal loop
	StopCh chan struct{}

	// Heap is the min-Heap to keep track of both tokens and leases
	Heap *vaultClientHeap

	// Config is the configuration to connect to vault
	Config *config.VaultConfig

	Lock   sync.RWMutex
	logger hclog.Logger
}

// RenewalRequest is a request object for renewal of both tokens and secret's
// leases.
type RenewalRequest struct {
	// ErrCh is the channel into which any renewal error will be sent to
	ErrCh chan error

	// ID is an identifier which represents either a token or a lease
	ID string

	// Increment is the duration for which the token or lease should be
	// renewed for
	Increment int

	// IsToken indicates whether the 'ID' field is a token or not
	IsToken bool
}

// Element representing an entry in the renewal heap
type vaultClientHeapEntry struct {
	req   *RenewalRequest
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

// NewVaultClient returns a new vault Vault from the given Config.
func NewVaultClient(config *config.VaultConfig, logger hclog.Logger, tokenDeriver TokenDeriverFunc) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("nil vault Config")
	}

	logger = logger.Named("vault").With("name", config.Name)

	c := &Client{
		Config: config,
		StopCh: make(chan struct{}),
		// Update channel should be a buffered channel
		UpdateCh:     make(chan struct{}, 1),
		Heap:         newVaultClientHeap(),
		logger:       logger,
		TokenDeriver: tokenDeriver,
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

	c.Vault = client

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

// IsTracked returns if a given identifier is already present in the Heap and
// hence is being renewed. Lock should be held before calling this method.
func (c *Client) IsTracked(id string) bool {
	if id == "" {
		return false
	}

	_, ok := c.Heap.heapMap[id]
	return ok
}

// isRunning returns true if the client is running.
func (c *Client) isRunning() bool {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	return c.Running
}

// Start starts the renewal loop of vault client
func (c *Client) Start() {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	if !c.Config.IsEnabled() || c.Running {
		return
	}

	c.Running = true

	go c.run()
}

// Stop stops the renewal loop of vault client
func (c *Client) Stop() {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	if !c.Config.IsEnabled() || !c.Running {
		return
	}

	c.Running = false
	close(c.StopCh)
}

// unlockAndUnset is used to unset the vault token on the client and release the
// lock. Helper method for deferring a call that does both.
func (c *Client) unlockAndUnset() {
	c.Vault.SetToken("")
	c.Lock.Unlock()
}

// DeriveToken takes in an allocation and a set of tasks and for each of the
// task, it derives a vault token from nomad server and unwraps it using vault.
// The return value is a map containing all the unwrapped tokens indexed by the
// task name.
func (c *Client) DeriveToken(alloc *structs.Allocation, taskNames []string) (map[string]string, error) {
	if !c.Config.IsEnabled() {
		return nil, fmt.Errorf("vault client not enabled")
	}
	if !c.isRunning() {
		return nil, fmt.Errorf("vault client is not running")
	}

	c.Lock.Lock()
	defer c.unlockAndUnset()

	// Use the token supplied to interact with vault
	c.Vault.SetToken("")

	tokens, err := c.TokenDeriver(alloc, taskNames, c.Vault)
	if err != nil {
		c.logger.Error("error deriving token", "error", err, "alloc_id", alloc.ID, "task_names", taskNames)
		return nil, err
	}

	return tokens, nil
}

// DeriveTokenWithJWT returns a Vault ACL token using the JWT login endpoint.
func (c *Client) DeriveTokenWithJWT(ctx context.Context, req JWTLoginRequest) (string, error) {
	if !c.Config.IsEnabled() {
		return "", fmt.Errorf("vault client not enabled")
	}
	if !c.isRunning() {
		return "", fmt.Errorf("vault client is not running")
	}

	c.Lock.Lock()
	defer c.unlockAndUnset()

	// Make sure the login request is not passing any token.
	c.Vault.SetToken("")

	jwtLoginPath := fmt.Sprintf("auth/%s/login", c.Config.JWTAuthBackendPath)
	s, err := c.Vault.Logical().WriteWithContext(ctx, jwtLoginPath,
		map[string]any{
			"role": req.Role,
			"jwt":  req.JWT,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to login with JWT: %w", err)
	}
	if s == nil {
		return "", errors.New("JWT login returned an empty secret")
	}
	if s.Auth == nil {
		return "", errors.New("JWT login did not return a token")
	}

	for _, w := range s.Warnings {
		c.logger.Warn("JWT login warning", "warning", w)
	}

	return s.Auth.ClientToken, nil
}

// GetConsulACL creates a vault API client and reads from vault a consul ACL
// token used by the task.
func (c *Client) GetConsulACL(token, path string) (*vaultapi.Secret, error) {
	if !c.Config.IsEnabled() {
		return nil, fmt.Errorf("vault client not enabled")
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if path == "" {
		return nil, fmt.Errorf("missing consul ACL token vault path")
	}

	c.Lock.Lock()
	defer c.unlockAndUnset()

	// Use the token supplied to interact with vault
	c.Vault.SetToken(token)

	// Read the consul ACL token and return the secret directly
	return c.Vault.Logical().Read(path)
}

// RenewToken renews the supplied token for a given duration (in seconds) and
// adds it to the min-heap so that it is renewed periodically by the renewal
// loop. Any error returned during renewal will be written to a buffered
// channel and the channel is returned instead of an actual error. This helps
// the caller be notified of a renewal failure asynchronously for appropriate
// actions to be taken. The caller of this function need not have to close the
// error channel.
func (c *Client) RenewToken(token string, increment int) (<-chan error, error) {
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
	renewalReq := &RenewalRequest{
		ErrCh:     errCh,
		ID:        token,
		IsToken:   true,
		Increment: increment,
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
func (c *Client) renew(req *RenewalRequest) error {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	if req == nil {
		return fmt.Errorf("nil renewal request")
	}
	if req.ErrCh == nil {
		return fmt.Errorf("renewal request error channel nil")
	}

	if !c.Config.IsEnabled() {
		close(req.ErrCh)
		return fmt.Errorf("vault client not enabled")
	}
	if !c.Running {
		close(req.ErrCh)
		return fmt.Errorf("vault client is not running")
	}
	if req.ID == "" {
		close(req.ErrCh)
		return fmt.Errorf("missing ID in renewal request")
	}
	if req.Increment < 1 {
		close(req.ErrCh)
		return fmt.Errorf("increment cannot be less than 1")
	}

	var renewalErr error
	// Set the token in the API client to the one that needs renewal
	leaseDuration := req.Increment
	if req.IsToken {
		c.Vault.SetToken(req.ID)

		// Renew the token
		renewResp, err := c.Vault.Auth().Token().RenewSelf(req.Increment)
		if err != nil {
			renewalErr = fmt.Errorf("failed to renew the vault token: %w", err)
		} else if renewResp == nil || renewResp.Auth == nil {
			renewalErr = fmt.Errorf("failed to renew the vault token")
		} else {
			// Don't set this if renewal fails
			leaseDuration = renewResp.Auth.LeaseDuration
		}

		// Reset the token in the API client before returning
		c.Vault.SetToken("")
	} else {
		// Renew the secret
		renewResp, err := c.Vault.Sys().Renew(req.ID, req.Increment)
		if err != nil {
			renewalErr = fmt.Errorf("failed to renew vault secret: %w", err)
		} else if renewResp == nil {
			renewalErr = fmt.Errorf("failed to renew vault secret")
		} else {
			// Don't set this if renewal fails
			leaseDuration = renewResp.LeaseDuration
		}
	}

	// Determine the next renewal time
	renewalDuration := RenewalTime(rand.Intn, leaseDuration)
	next := time.Now().Add(renewalDuration)

	fatal := false
	if renewalErr != nil &&
		(strings.Contains(renewalErr.Error(), "lease not found or lease is not renewable") ||
			strings.Contains(renewalErr.Error(), "lease is not renewable") ||
			strings.Contains(renewalErr.Error(), "token not found") ||
			strings.Contains(renewalErr.Error(), "permission denied")) {
		fatal = true
	} else if renewalErr != nil {
		c.logger.Debug("renewal error details", "req.Increment", req.Increment, "lease_duration", leaseDuration, "renewal_duration", renewalDuration)
		c.logger.Error("error during renewal of lease or token failed due to a non-fatal error; retrying",
			"error", renewalErr, "period", next)
	}

	if c.IsTracked(req.ID) {
		if fatal {
			// If encountered with an error where in a lease or a
			// token is not valid at all with vault, and if that
			// item is tracked by the renewal loop, stop renewing
			// it by removing the corresponding heap entry.
			if err := c.Heap.Remove(req.ID); err != nil {
				return fmt.Errorf("failed to remove heap entry: %w", err)
			}

			// Report the fatal error to the client
			req.ErrCh <- renewalErr
			close(req.ErrCh)

			return renewalErr
		}

		// If the identifier is already tracked, this indicates a
		// subsequest renewal. In this case, update the existing
		// element in the heap with the new renewal time.
		if err := c.Heap.Update(req, next); err != nil {
			return fmt.Errorf("failed to update heap entry. err: %w", err)
		}

		// There is no need to signal an update to the renewal loop
		// here because this case is hit from the renewal loop itself.
	} else {
		if fatal {
			// If encountered with an error where in a lease or a
			// token is not valid at all with vault, and if that
			// item is not tracked by renewal loop, don't add it.

			// Report the fatal error to the client
			req.ErrCh <- renewalErr
			close(req.ErrCh)

			return renewalErr
		}

		// If the identifier is not already tracked, this is a first
		// renewal request. In this case, add an entry into the heap
		// with the next renewal time.
		if err := c.Heap.Push(req, next); err != nil {
			return fmt.Errorf("failed to push an entry to heap.  err: %w", err)
		}

		// Signal an update for the renewal loop to trigger a fresh
		// computation for the next best candidate for renewal.
		if c.Running {
			select {
			case c.UpdateCh <- struct{}{}:
			default:
			}
		}
	}

	return nil
}

// run is the renewal loop which performs the periodic renewals of both the
// tokens and the secret leases.
func (c *Client) run() {
	if !c.Config.IsEnabled() {
		return
	}

	var renewalCh <-chan time.Time
	for c.Config.IsEnabled() && c.isRunning() {
		// Fetches the candidate for next renewal
		renewalReq, renewalTime := c.NextRenewal()
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
		case <-c.UpdateCh:
			continue
		case <-c.StopCh:
			c.logger.Debug("stopped")
			return
		}
	}
}

// StopRenewToken removes the item from the heap which represents the given
// token.
func (c *Client) StopRenewToken(token string) error {
	return c.stopRenew(token)
}

// stopRenew removes the given identifier from the heap and signals the renewal
// loop to compute the next best candidate for renewal.
func (c *Client) stopRenew(id string) error {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	if !c.IsTracked(id) {
		return nil
	}

	if err := c.Heap.Remove(id); err != nil {
		return fmt.Errorf("failed to remove Heap entry: %w", err)
	}

	// Signal an update to the renewal loop.
	if c.Running {
		select {
		case c.UpdateCh <- struct{}{}:
		default:
		}
	}

	return nil
}

// NextRenewal returns the root element of the min-heap, which represents the
// next element to be renewed and the time at which the renewal needs to be
// triggered.
func (c *Client) NextRenewal() (*RenewalRequest, time.Time) {
	c.Lock.RLock()
	defer c.Lock.RUnlock()

	if c.Heap.Length() == 0 {
		return nil, time.Time{}
	}

	// Fetches the root element in the min-heap
	nextEntry := c.Heap.Peek()
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
func (h *vaultClientHeap) Push(req *RenewalRequest, next time.Time) error {
	if req == nil {
		return fmt.Errorf("nil request")
	}

	if _, ok := h.heapMap[req.ID]; ok {
		return fmt.Errorf("entry %v already exists", req.ID)
	}

	heapEntry := &vaultClientHeapEntry{
		req:  req,
		next: next,
	}
	h.heapMap[req.ID] = heapEntry
	heap.Push(&h.heap, heapEntry)
	return nil
}

// Update will modify the existing item in the heap with the new data and the
// time, and fixes the heap.
func (h *vaultClientHeap) Update(req *RenewalRequest, next time.Time) error {
	if entry, ok := h.heapMap[req.ID]; ok {
		entry.req = req
		entry.next = next
		heap.Fix(&h.heap, entry.index)
		return nil
	}

	return fmt.Errorf("heap doesn't contain %v", req.ID)
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

// RenewalTime returns when a token should be renewed given its leaseDuration
// and a randomizer to provide jitter.
//
// Leases < 1m will be not jitter.
func RenewalTime(dice randIntn, leaseDuration int) time.Duration {
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
