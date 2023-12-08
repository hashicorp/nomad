// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/useragent"
	tomb "gopkg.in/tomb.v2"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

const (
	// vaultTokenCreateTTL is the duration the wrapped token for the client is
	// valid for. The units are in seconds.
	vaultTokenCreateTTL = "60s"

	// minimumTokenTTL is the minimum Token TTL allowed for child tokens.
	minimumTokenTTL = 5 * time.Minute

	// defaultTokenTTL is the default Token TTL used when the passed token is a
	// root token such that child tokens aren't being created against a role
	// that has defined a TTL
	defaultTokenTTL = "72h"

	// requestRateLimit is the maximum number of requests per second Nomad will
	// make against Vault
	requestRateLimit rate.Limit = 500.0

	// maxParallelRevokes is the maximum number of parallel Vault
	// token revocation requests
	maxParallelRevokes = 64

	// vaultRevocationIntv is the interval at which Vault tokens that failed
	// initial revocation are retried
	vaultRevocationIntv = 5 * time.Minute

	// vaultCapabilitiesLookupPath is the path to lookup the capabilities of
	// ones token.
	vaultCapabilitiesLookupPath = "sys/capabilities-self"

	// vaultTokenRenewPath is the path used to renew our token
	vaultTokenRenewPath = "auth/token/renew-self"

	// vaultTokenLookupPath is the path used to lookup a token
	vaultTokenLookupPath = "auth/token/lookup"

	// vaultTokenRevokePath is the path used to revoke a token
	vaultTokenRevokePath = "auth/token/revoke-accessor"

	// vaultRoleLookupPath is the path to lookup a role
	vaultRoleLookupPath = "auth/token/roles/%s"

	// vaultRoleCreatePath is the path to create a token from a role
	vaultTokenRoleCreatePath = "auth/token/create/%s"
)

var (
	// vaultCapabilitiesCapability is the expected capability of Nomad's Vault
	// token on the the path. The token must have at least one of the
	// capabilities.
	vaultCapabilitiesCapability = []string{"update", "root"}

	// vaultTokenRenewCapability is the expected capability Nomad's
	// Vault token should have on the path. The token must have at least one of
	// the capabilities.
	vaultTokenRenewCapability = []string{"update", "root"}

	// vaultTokenLookupCapability is the expected capability Nomad's
	// Vault token should have on the path. The token must have at least one of
	// the capabilities.
	vaultTokenLookupCapability = []string{"update", "root"}

	// vaultTokenRevokeCapability is the expected capability Nomad's
	// Vault token should have on the path. The token must have at least one of
	// the capabilities.
	vaultTokenRevokeCapability = []string{"update", "root"}

	// vaultRoleLookupCapability is the the expected capability Nomad's Vault
	// token should have on the path. The token must have at least one of the
	// capabilities.
	vaultRoleLookupCapability = []string{"read", "root"}

	// vaultTokenRoleCreateCapability is the the expected capability Nomad's Vault
	// token should have on the path. The token must have at least one of the
	// capabilities.
	vaultTokenRoleCreateCapability = []string{"update", "root"}
)

// VaultClient is the Servers interface for interfacing with Vault
type VaultClient interface {
	// SetActive activates or de-activates the Vault client. When active, token
	// creation/lookup/revocation operation are allowed.
	SetActive(active bool)

	// SetConfig updates the config used by the Vault client
	SetConfig(config *config.VaultConfig) error

	// GetConfig returns a copy of the config used by the Vault client, for
	// testing
	GetConfig() *config.VaultConfig

	// CreateToken takes an allocation and task and returns an appropriate Vault
	// Secret
	CreateToken(ctx context.Context, a *structs.Allocation, task string) (*vapi.Secret, error)

	// LookupToken takes a token string and returns its capabilities.
	LookupToken(ctx context.Context, token string) (*vapi.Secret, error)

	// RevokeTokens takes a set of tokens accessor and revokes the tokens
	RevokeTokens(ctx context.Context, accessors []*structs.VaultAccessor, committed bool) error

	// MarkForRevocation revokes the tokens in background
	MarkForRevocation(accessors []*structs.VaultAccessor) error

	// Stop is used to stop token renewal
	Stop()

	// Running returns whether the Vault client is running
	Running() bool

	// Stats returns the Vault clients statistics
	Stats() map[string]string

	// EmitStats emits that clients statistics at the given period until stopCh
	// is called.
	EmitStats(period time.Duration, stopCh <-chan struct{})
}

// VaultStats returns all the stats about Vault tokens created and managed by
// Nomad.
type VaultStats struct {
	// TrackedForRevoke is the count of tokens that are being tracked to be
	// revoked since they could not be immediately revoked.
	TrackedForRevoke int

	// TokenTTL is the time-to-live duration for the current token
	TokenTTL time.Duration

	// TokenExpiry is the recorded expiry time of the current token
	TokenExpiry time.Time

	// LastRenewalTime is the time since the token was last renewed
	LastRenewalTime     time.Time
	TimeFromLastRenewal time.Duration

	// NextRenewalTime is the time the token will attempt to renew
	NextRenewalTime   time.Time
	TimeToNextRenewal time.Duration
}

// PurgeVaultAccessorFn is called to remove VaultAccessors from the system. If
// the function returns an error, the token will still be tracked and revocation
// will retry till there is a success
type PurgeVaultAccessorFn func(accessors []*structs.VaultAccessor) error

// vaultClient is the Servers implementation of the VaultClient interface. The
// client renews the PeriodicToken given in the Vault configuration and provides
// the Server with the ability to create child tokens and lookup the permissions
// of tokens.
type vaultClient struct {
	// limiter is used to rate limit requests to Vault
	limiter *rate.Limiter

	// client is the Vault API client used for Namespace-relative integrations
	// with the Vault API (anything except `/v1/sys`). If this server is not
	// configured to reference a Vault namespace, this will point to the same
	// client as clientSys
	client *vapi.Client

	// clientSys is the Vault API client used for non-Namespace-relative integrations
	// with the Vault API (anything involving `/v1/sys`). This client is never configured
	// with a Vault namespace, because these endpoints may return errors if a namespace
	// header is provided
	clientSys *vapi.Client

	// auth is the Vault token auth API client
	auth *vapi.TokenAuth

	// config is the user passed Vault config
	config *config.VaultConfig

	// connEstablished marks whether we have an established connection to Vault.
	connEstablished bool

	// connEstablishedErr marks an error that can occur when establishing a
	// connection
	connEstablishedErr error

	// token is the raw token used by the client
	token string

	// tokenData is the data of the passed Vault token
	tokenData *structs.VaultTokenData

	// revoking tracks the VaultAccessors that must be revoked
	revoking map[*structs.VaultAccessor]time.Time
	purgeFn  PurgeVaultAccessorFn
	revLock  sync.Mutex

	// active indicates whether the vaultClient is active. It should be
	// accessed using a helper and updated atomically
	active int32

	// running indicates whether the vault client is started.
	running bool

	// renewLoopActive indicates whether the renewal goroutine is running
	// It should be accessed and updated atomically
	// used for testing purposes only
	renewLoopActive int32

	// childTTL is the TTL for child tokens.
	childTTL string

	// currentExpiration is the time the current token lease expires
	currentExpiration     time.Time
	currentExpirationLock sync.Mutex
	lastRenewalTime       time.Time
	nextRenewalTime       time.Time
	renewalTimeLock       sync.Mutex

	tomb   *tomb.Tomb
	logger log.Logger

	// l is used to lock the configuration aspects of the client such that
	// multiple callers can't cause conflicting config updates
	l sync.Mutex

	// setConfigLock serializes access to the SetConfig method
	setConfigLock sync.Mutex

	// consts as struct fields for overriding in tests
	maxRevokeBatchSize int
	revocationIntv     time.Duration

	entHandler taskClientHandler
}

type taskClientHandler interface {
	clientForTask(v *vaultClient, namespace string) (*vapi.Client, error)
}

// NewVaultClient returns a Vault client from the given config. If the client
// couldn't be made an error is returned.
func NewVaultClient(c *config.VaultConfig, logger log.Logger, purgeFn PurgeVaultAccessorFn, delegate taskClientHandler) (*vaultClient, error) {
	if c == nil {
		return nil, fmt.Errorf("must pass valid VaultConfig")
	}

	if logger == nil {
		return nil, fmt.Errorf("must pass valid logger")
	}
	if purgeFn == nil {
		purgeFn = func(accessors []*structs.VaultAccessor) error { return nil }
	}
	if delegate == nil {
		delegate = &VaultNoopDelegate{}
	}

	v := &vaultClient{
		config:             c,
		logger:             logger.Named("vault"),
		limiter:            rate.NewLimiter(requestRateLimit, int(requestRateLimit)),
		revoking:           make(map[*structs.VaultAccessor]time.Time),
		purgeFn:            purgeFn,
		tomb:               &tomb.Tomb{},
		maxRevokeBatchSize: maxVaultRevokeBatchSize,
		revocationIntv:     vaultRevocationIntv,
		entHandler:         delegate,
	}

	if v.config.IsEnabled() {
		if err := v.buildClient(); err != nil {
			return nil, err
		}

		// Launch the required goroutines
		v.tomb.Go(wrapNilError(v.establishConnection))
		v.tomb.Go(wrapNilError(v.revokeDaemon))

		v.running = true
	}

	return v, nil
}

func (v *vaultClient) Stop() {
	v.l.Lock()
	running := v.running
	v.running = false
	v.l.Unlock()

	if running {
		v.tomb.Kill(nil)
		v.tomb.Wait()
		v.flush()
	}
}

func (v *vaultClient) Running() bool {
	v.l.Lock()
	defer v.l.Unlock()
	return v.running
}

// SetActive activates or de-activates the Vault client. When active, token
// creation/lookup/revocation operation are allowed. All queued revocations are
// cancelled if set un-active as it is assumed another instances is taking over
func (v *vaultClient) SetActive(active bool) {
	if active {
		atomic.StoreInt32(&v.active, 1)
	} else {
		atomic.StoreInt32(&v.active, 0)
	}

	// Clear out the revoking tokens
	v.revLock.Lock()
	v.revoking = make(map[*structs.VaultAccessor]time.Time)
	v.revLock.Unlock()
}

// flush is used to reset the state of the vault client
func (v *vaultClient) flush() {
	v.l.Lock()
	defer v.l.Unlock()
	v.revLock.Lock()
	defer v.revLock.Unlock()

	v.client = nil
	v.clientSys = nil
	v.auth = nil
	v.connEstablished = false
	v.connEstablishedErr = nil
	v.token = ""
	v.tokenData = nil
	v.revoking = make(map[*structs.VaultAccessor]time.Time)
	v.childTTL = ""
	v.tomb = &tomb.Tomb{}
}

// GetConfig returns a copy of this vault client's configuration, for testing.
func (v *vaultClient) GetConfig() *config.VaultConfig {
	v.setConfigLock.Lock()
	defer v.setConfigLock.Unlock()
	return v.config.Copy()
}

// SetConfig is used to update the Vault config being used. A temporary outage
// may occur after calling as it re-establishes a connection to Vault
func (v *vaultClient) SetConfig(config *config.VaultConfig) error {
	if config == nil {
		return fmt.Errorf("must pass valid VaultConfig")
	}
	v.setConfigLock.Lock()
	defer v.setConfigLock.Unlock()

	v.l.Lock()
	defer v.l.Unlock()

	// If reloading the same config, no-op
	if v.config.Equal(config) {
		return nil
	}

	// Kill any background routines
	if v.running {
		// Kill any background routine
		v.tomb.Kill(nil)

		// Locking around tomb.Wait can deadlock with
		// establishConnection exiting, so we must unlock here.
		v.l.Unlock()
		v.tomb.Wait()
		v.l.Lock()

		// Stop accepting any new requests
		v.connEstablished = false
		v.tomb = &tomb.Tomb{}
		v.running = false
	}

	// Store the new config
	v.config = config

	// Check if we should relaunch
	if v.config.IsEnabled() {
		// Rebuild the client
		if err := v.buildClient(); err != nil {
			return err
		}

		// Launch the required goroutines
		v.tomb.Go(wrapNilError(v.establishConnection))
		v.tomb.Go(wrapNilError(v.revokeDaemon))
		v.running = true
	}

	return nil
}

// buildClient is used to build a Vault client based on the stored Vault config
func (v *vaultClient) buildClient() error {
	// Validate we have the required fields.
	if v.config.Token == "" {
		return errors.New("Vault token must be set")
	} else if v.config.Addr == "" {
		return errors.New("Vault address must be set")
	}

	// Parse the TTL if it is set
	if v.config.TaskTokenTTL != "" {
		d, err := time.ParseDuration(v.config.TaskTokenTTL)
		if err != nil {
			return fmt.Errorf("failed to parse TaskTokenTTL %q: %v", v.config.TaskTokenTTL, err)
		}

		if d.Nanoseconds() < minimumTokenTTL.Nanoseconds() {
			return fmt.Errorf("ChildTokenTTL is less than minimum allowed of %v", minimumTokenTTL)
		}

		v.childTTL = v.config.TaskTokenTTL
	} else {
		// Default the TaskTokenTTL
		v.childTTL = defaultTokenTTL
	}

	// Get the Vault API configuration
	apiConf, err := v.config.ApiConfig()
	if err != nil {
		return fmt.Errorf("Failed to create Vault API config: %v", err)
	}

	// Create the Vault API client
	client, err := vapi.NewClient(apiConf)
	if err != nil {
		v.logger.Error("failed to create Vault client and not retrying", "error", err)
		return err
	}
	useragent.SetHeaders(client)

	// Store the client, create/assign the /sys client
	v.client = client
	if v.config.Namespace != "" {
		v.logger.Debug("configuring Vault namespace", "namespace", v.config.Namespace)
		v.clientSys, err = vapi.NewClient(apiConf)
		if err != nil {
			v.logger.Error("failed to create Vault sys client and not retrying", "error", err)
			return err
		}
		useragent.SetHeaders(v.clientSys)
		client.SetNamespace(v.config.Namespace)
	} else {
		v.clientSys = client
	}

	// Set the token
	v.token = v.config.Token
	client.SetToken(v.token)
	v.auth = client.Auth().Token()

	return nil
}

// establishConnection is used to make first contact with Vault. This should be
// called in a go-routine since the connection is retried until the Vault Client
// is stopped or the connection is successfully made at which point the renew
// loop is started.
func (v *vaultClient) establishConnection() {
	// Create the retry timer and set initial duration to zero so it fires
	// immediately
	retryTimer := time.NewTimer(0)
	initStatus := false
OUTER:
	for {
		select {
		case <-v.tomb.Dying():
			return
		case <-retryTimer.C:
			// Retry validating the token till success
			if err := v.parseSelfToken(); err != nil {
				// if parsing token fails, try to distinguish legitimate token error from transient Vault initialization/connection issue
				if !initStatus {
					if _, err := v.clientSys.Sys().Health(); err != nil {
						v.logger.Warn("failed to contact Vault API", "retry", v.config.ConnectionRetryIntv, "error", err)
						retryTimer.Reset(v.config.ConnectionRetryIntv)
						continue OUTER
					}
					initStatus = true
				}

				v.logger.Error("failed to validate self token/role", "retry", v.config.ConnectionRetryIntv, "error", err)
				retryTimer.Reset(v.config.ConnectionRetryIntv)
				v.l.Lock()
				v.connEstablished = true
				v.connEstablishedErr = fmt.Errorf("failed to establish connection to Vault: %v", err)
				v.l.Unlock()
				continue OUTER
			}

			break OUTER
		}
	}

	// Set the wrapping function such that token creation is wrapped now
	// that we know our role
	v.client.SetWrappingLookupFunc(v.getWrappingFn())

	// If we are given a non-root token, start renewing it
	if v.tokenData.Root() && v.tokenData.CreationTTL == 0 {
		v.logger.Debug("not renewing token as it is root")
	} else {
		v.logger.Debug("starting renewal loop", "creation_ttl", time.Duration(v.tokenData.CreationTTL)*time.Second)
		v.tomb.Go(wrapNilError(v.renewalLoop))
	}

	v.l.Lock()
	v.connEstablished = true
	v.connEstablishedErr = nil
	v.l.Unlock()
}

func (v *vaultClient) isRenewLoopActive() bool {
	return atomic.LoadInt32(&v.renewLoopActive) == 1
}

// renewalLoop runs the renew loop. This should only be called if we are given a
// non-root token.
func (v *vaultClient) renewalLoop() {
	atomic.StoreInt32(&v.renewLoopActive, 1)
	defer atomic.StoreInt32(&v.renewLoopActive, 0)

	// Create the renewal timer and set initial duration to zero so it fires
	// immediately
	authRenewTimer := time.NewTimer(0)

	// Backoff is to reduce the rate we try to renew with Vault under error
	// situations
	backoff := 0.0

	for {
		select {
		case <-v.tomb.Dying():
			return
		case <-authRenewTimer.C:
			// Renew the token and determine the new expiration
			recoverable, err := v.renew()
			v.currentExpirationLock.Lock()
			currentExpiration := v.currentExpiration
			v.currentExpirationLock.Unlock()

			// Successfully renewed
			if err == nil {
				// Attempt to renew the token at half the expiration time
				durationUntilRenew := time.Until(currentExpiration) / 2
				v.renewalTimeLock.Lock()
				now := time.Now()
				v.lastRenewalTime = now
				v.nextRenewalTime = now.Add(durationUntilRenew)
				v.renewalTimeLock.Unlock()

				v.logger.Info("successfully renewed token", "next_renewal", durationUntilRenew)
				authRenewTimer.Reset(durationUntilRenew)

				// Reset any backoff
				backoff = 0
				break
			}

			metrics.IncrCounter([]string{"nomad", "vault", "renew_failed"}, 1)
			v.logger.Warn("got error or bad auth, so backing off", "error", err, "recoverable", recoverable)

			if !recoverable {
				return
			}

			backoff = nextBackoff(backoff, currentExpiration)
			if backoff < 0 {
				// We have failed to renew the token past its expiration. Stop
				// renewing with Vault.
				v.logger.Error("failed to renew Vault token before lease expiration. Shutting down Vault client",
					"error", err)
				v.l.Lock()
				v.connEstablished = false
				v.connEstablishedErr = err
				v.l.Unlock()
				return
			}

			durationUntilRetry := time.Duration(backoff) * time.Second
			v.renewalTimeLock.Lock()
			v.nextRenewalTime = time.Now().Add(durationUntilRetry)
			v.renewalTimeLock.Unlock()
			v.logger.Info("backing off renewal", "retry", durationUntilRetry)

			authRenewTimer.Reset(durationUntilRetry)
		}
	}
}

// nextBackoff returns the delay for the next auto renew interval, in seconds.
// Returns negative value if past expiration
//
// It should increase the amount of backoff each time, with the following rules:
//
//   - If token expired already despite earlier renewal attempts,
//     back off for 1 minute + jitter
//   - If we have an existing authentication that is going to expire,
//
// never back off more than half of the amount of time remaining
// until expiration (with 5s floor)
// * Never back off more than 30 seconds multiplied by a random
// value between 1 and 2
// * Use randomness so that many clients won't keep hitting Vault
// at the same time
func nextBackoff(backoff float64, expiry time.Time) float64 {
	maxBackoff := time.Until(expiry) / 2

	if maxBackoff < 0 {
		// expiry passed
		return 60 * (1.0 + rand.Float64())
	}

	switch {
	case backoff >= 24:
		backoff = 30
	default:
		backoff = backoff * 1.25
	}

	// Add randomness
	backoff = backoff * (1.0 + rand.Float64())

	if backoff > maxBackoff.Seconds() {
		backoff = maxBackoff.Seconds()
	}

	if backoff < 5 {
		backoff = 5
	}

	return backoff
}

// renew attempts to renew our Vault token. If the renewal fails, an error is
// returned.  The boolean indicates whether it's safe to attempt to renew again.
// This method updates the currentExpiration time
func (v *vaultClient) renew() (bool, error) {
	// Track how long the request takes
	defer metrics.MeasureSince([]string{"nomad", "vault", "renew"}, time.Now())

	// Attempt to renew the token
	secret, err := v.auth.RenewSelf(v.tokenData.CreationTTL)
	if err != nil {
		// Check if there is a permission denied
		recoverable := !structs.VaultUnrecoverableError.MatchString(err.Error())
		return recoverable, fmt.Errorf("failed to renew the vault token: %v", err)
	}

	if secret == nil {
		// It's possible for RenewSelf to return (nil, nil) if the
		// response body from Vault is empty.
		return true, fmt.Errorf("renewal failed: empty response from vault")
	}

	// these treated as transient errors, where can keep renewing
	auth := secret.Auth
	if auth == nil {
		return true, fmt.Errorf("renewal successful but not auth information returned")
	} else if auth.LeaseDuration == 0 {
		return true, fmt.Errorf("renewal successful but no lease duration returned")
	}

	v.extendExpiration(auth.LeaseDuration)

	v.logger.Debug("successfully renewed server token")
	return true, nil
}

// getWrappingFn returns an appropriate wrapping function for Nomad Servers
func (v *vaultClient) getWrappingFn() func(operation, path string) string {
	createPath := "auth/token/create"
	role := v.getRole()
	if role != "" {
		createPath = fmt.Sprintf("auth/token/create/%s", role)
	}

	return func(operation, path string) string {
		// Only wrap the token create operation
		if operation != "POST" || path != createPath {
			return ""
		}

		return vaultTokenCreateTTL
	}
}

// parseSelfToken looks up the Vault token in Vault and parses its data storing
// it in the client. If the token is not valid for Nomads purposes an error is
// returned.
func (v *vaultClient) parseSelfToken() error {
	// Try looking up the token using the self endpoint
	secret, err := v.lookupSelf()
	if err != nil {
		return err
	}

	// Read and parse the fields
	var data structs.VaultTokenData
	if err := structs.DecodeVaultSecretData(secret, &data); err != nil {
		return fmt.Errorf("failed to parse Vault token's data block: %v", err)
	}
	v.tokenData = &data
	v.extendExpiration(data.TTL)

	// The criteria that must be met for the token to be valid are as follows:
	// 1) If token is non-root or is but has a creation ttl
	//   a) The token must be renewable
	//   b) Token must have a non-zero TTL
	// 2) Must have update capability for "auth/token/lookup/" (used to verify incoming tokens)
	// 3) Must have update capability for "/auth/token/revoke-accessor/" (used to revoke unneeded tokens)
	// 4) If configured to create tokens against a role:
	//   a) Must have read capability for "auth/token/roles/<role_name" (Can just attempt a read)
	//   b) Must have update capability for path "auth/token/create/<role_name>"
	//   c) Role must:
	//     1) Must allow tokens to be renewed
	//     2) Must not have an explicit max TTL
	//     3) Must have non-zero period
	// 5) If not configured against a role, the token must be root

	var mErr multierror.Error
	role := v.getRole()
	if !data.Root() {
		// All non-root tokens must be renewable
		if !data.Renewable {
			_ = multierror.Append(&mErr, fmt.Errorf("Vault token is not renewable or root"))
		}

		// All non-root tokens must have a lease duration
		if data.CreationTTL == 0 {
			_ = multierror.Append(&mErr, fmt.Errorf("invalid lease duration of zero"))
		}

		// The lease duration can not be expired
		if data.TTL == 0 {
			_ = multierror.Append(&mErr, fmt.Errorf("token TTL is zero"))
		}

		// There must be a valid role since we aren't root
		if role == "" {
			_ = multierror.Append(&mErr, fmt.Errorf("token role name must be set when not using a root token"))
		}

	} else if data.CreationTTL != 0 {
		// If the root token has a TTL it must be renewable
		if !data.Renewable {
			_ = multierror.Append(&mErr, fmt.Errorf("Vault token has a TTL but is not renewable"))
		} else if data.TTL == 0 {
			// If the token has a TTL make sure it has not expired
			_ = multierror.Append(&mErr, fmt.Errorf("token TTL is zero"))
		}
	}

	// Check we have the correct capabilities
	if err := v.validateCapabilities(role, data.Root()); err != nil {
		_ = multierror.Append(&mErr, err)
	}

	// If given a role validate it
	if role != "" {
		if err := v.validateRole(role); err != nil {
			_ = multierror.Append(&mErr, err)
		}
	}

	return mErr.ErrorOrNil()
}

// lookupSelf is a helper function that looks up latest self lease info.
func (v *vaultClient) lookupSelf() (*vapi.Secret, error) {
	// Get the initial lease duration
	auth := v.client.Auth().Token()

	secret, err := auth.LookupSelf()
	if err == nil && secret != nil && secret.Data != nil {
		return secret, nil
	}

	// Try looking up our token directly, even when we get an empty response,
	// in case of an unexpected event - a true failure would occur in this lookup again
	secret, err = auth.Lookup(v.client.Token())
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to lookup Vault periodic token: %v", err)
	case secret == nil || secret.Data == nil:
		return nil, fmt.Errorf("failed to lookup Vault periodic token: got empty response")
	default:
		return secret, nil
	}
}

// getRole returns the role name to be used when creating tokens
func (v *vaultClient) getRole() string {
	if v.config.Role != "" {
		return v.config.Role
	}

	return v.tokenData.Role
}

// validateCapabilities checks that Nomad's Vault token has the correct
// capabilities.
func (v *vaultClient) validateCapabilities(role string, root bool) error {
	// Check if the token can lookup capabilities.
	var mErr multierror.Error
	_, _, err := v.hasCapability(vaultCapabilitiesLookupPath, vaultCapabilitiesCapability)
	if err != nil {
		// Check if there is a permission denied
		if structs.VaultUnrecoverableError.MatchString(err.Error()) {
			// Since we can't read permissions, we just log a warning that we
			// can't tell if the Vault token will work
			msg := fmt.Sprintf("can not lookup token capabilities. "+
				"As such certain operations may fail in the future. "+
				"Please give Nomad a Vault token with one of the following "+
				"capabilities %q on %q so that the required capabilities can be verified",
				vaultCapabilitiesCapability, vaultCapabilitiesLookupPath)
			v.logger.Warn(msg)
			return nil
		} else {
			_ = multierror.Append(&mErr, err)
		}
	}

	// verify is a helper function that verifies the token has one of the
	// capabilities on the given path and adds an issue to the error
	verify := func(path string, requiredCaps []string) {
		ok, caps, err := v.hasCapability(path, requiredCaps)
		if err != nil {
			_ = multierror.Append(&mErr, err)
		} else if !ok {
			_ = multierror.Append(&mErr,
				fmt.Errorf("token must have one of the following capabilities %q on %q; has %v", requiredCaps, path, caps))
		}
	}

	// Check if we are verifying incoming tokens
	if !v.config.AllowsUnauthenticated() {
		verify(vaultTokenLookupPath, vaultTokenLookupCapability)
	}

	// Verify we can renew our selves tokens
	verify(vaultTokenRenewPath, vaultTokenRenewCapability)

	// Verify we can revoke tokens
	verify(vaultTokenRevokePath, vaultTokenRevokeCapability)

	// If we are using a role verify the capability
	if role != "" {
		// Verify we can read the role
		verify(fmt.Sprintf(vaultRoleLookupPath, role), vaultRoleLookupCapability)

		// Verify we can create from the role
		verify(fmt.Sprintf(vaultTokenRoleCreatePath, role), vaultTokenRoleCreateCapability)
	}

	return mErr.ErrorOrNil()
}

// hasCapability takes a path and returns whether the token has at least one of
// the required capabilities on the given path. It also returns the set of
// capabilities the token does have as well as any error that occurred.
func (v *vaultClient) hasCapability(path string, required []string) (bool, []string, error) {
	caps, err := v.client.Sys().CapabilitiesSelf(path)
	if err != nil {
		return false, nil, err
	}
	for _, c := range caps {
		for _, r := range required {
			if c == r {
				return true, caps, nil
			}
		}
	}
	return false, caps, nil
}

// validateRole contacts Vault and checks that the given Vault role is valid for
// the purposes of being used by Nomad
func (v *vaultClient) validateRole(role string) error {
	if role == "" {
		return fmt.Errorf("Invalid empty role name")
	}

	// Validate the role
	rsecret, err := v.client.Logical().Read(fmt.Sprintf("auth/token/roles/%s", role))
	if err != nil {
		return fmt.Errorf("failed to lookup role %q: %v", role, err)
	}
	if rsecret == nil {
		return fmt.Errorf("Role %q does not exist", role)
	}

	// Read and parse the fields
	var data structs.VaultTokenRoleData
	if err := structs.DecodeVaultSecretData(rsecret, &data); err != nil {
		return fmt.Errorf("failed to parse Vault role's data block: %v", err)
	}

	// Validate the role is acceptable
	var mErr multierror.Error
	if !data.Renewable {
		_ = multierror.Append(&mErr, fmt.Errorf("Role must allow tokens to be renewed"))
	}

	if data.ExplicitMaxTtl != 0 || data.TokenExplicitMaxTtl != 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Role can not use an explicit max ttl. Token must be periodic."))
	}

	if data.Period == 0 && data.TokenPeriod == 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Role must have a non-zero period to make tokens periodic."))
	}

	return mErr.ErrorOrNil()
}

// ConnectionEstablished returns whether a connection to Vault has been
// established and any error that potentially caused it to be false
func (v *vaultClient) ConnectionEstablished() (bool, error) {
	v.l.Lock()
	defer v.l.Unlock()
	return v.connEstablished, v.connEstablishedErr
}

// Enabled returns whether the client is active
func (v *vaultClient) Enabled() bool {
	v.l.Lock()
	defer v.l.Unlock()
	return v.config.IsEnabled()
}

// Active returns whether the client is active
func (v *vaultClient) Active() bool {
	return atomic.LoadInt32(&v.active) == 1
}

// CreateToken takes the allocation and task and returns an appropriate Vault
// token. The call is rate limited and may be canceled with the passed policy.
// When the error is recoverable, it will be of type RecoverableError
func (v *vaultClient) CreateToken(ctx context.Context, a *structs.Allocation, task string) (*vapi.Secret, error) {
	if !v.Enabled() {
		return nil, fmt.Errorf("Vault integration disabled")
	}
	if !v.Active() {
		return nil, structs.NewRecoverableError(fmt.Errorf("Vault client not active"), true)
	}
	// Check if we have established a connection with Vault
	if established, err := v.ConnectionEstablished(); !established && err == nil {
		return nil, structs.NewRecoverableError(fmt.Errorf("Connection to Vault has not been established"), true)
	} else if err != nil {
		return nil, err
	}

	// Track how long the request takes
	defer metrics.MeasureSince([]string{"nomad", "vault", "create_token"}, time.Now())

	// Retrieve the Vault block for the task
	vaultBlocks := a.Job.Vault()
	if vaultBlocks == nil {
		return nil, fmt.Errorf("Job does not require Vault token")
	}
	tg, ok := vaultBlocks[a.TaskGroup]
	if !ok {
		return nil, fmt.Errorf("Task group does not require Vault token")
	}
	taskVault, ok := tg[task]
	if !ok {
		return nil, fmt.Errorf("Task does not require Vault token")
	}

	// Set namespace for task
	namespaceForTask := v.config.Namespace
	if taskVault.Namespace != "" {
		namespaceForTask = taskVault.Namespace
	}

	// Build the creation request
	req := &vapi.TokenCreateRequest{
		Policies: taskVault.Policies,
		Metadata: map[string]string{
			"AllocationID": a.ID,
			"JobID":        a.JobID,
			"TaskGroup":    a.TaskGroup,
			"Task":         task,
			"NodeID":       a.NodeID,
			"Namespace":    namespaceForTask,
		},
		TTL:         v.childTTL,
		DisplayName: fmt.Sprintf("%s-%s", a.ID, task),
	}

	// Ensure we are under our rate limit
	if err := v.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Make the request and switch depending on whether we are using a root
	// token or a role based token
	var secret *vapi.Secret
	var err error
	role := v.getRole()

	// Fetch client for task
	taskClient, err := v.entHandler.clientForTask(v, namespaceForTask)
	if err != nil {
		return nil, err
	}

	if v.tokenData.Root() && role == "" {
		req.Period = v.childTTL
		secret, err = taskClient.Auth().Token().Create(req)
	} else {
		// Make the token using the role
		secret, err = taskClient.Auth().Token().CreateWithRole(req, role)
	}

	// Determine whether it is unrecoverable
	if err != nil {
		err = fmt.Errorf("failed to create an alloc vault token: %v", err)
		if structs.VaultUnrecoverableError.MatchString(err.Error()) {
			return secret, err
		}

		// The error is recoverable
		return nil, structs.NewRecoverableError(err, true)
	}

	// Validate the response
	var validationErr error
	if secret == nil {
		validationErr = fmt.Errorf("Vault returned nil Secret")
	} else if secret.WrapInfo == nil {
		validationErr = fmt.Errorf("Vault returned Secret with nil WrapInfo. Secret warnings: %v", secret.Warnings)
	} else if secret.WrapInfo.WrappedAccessor == "" {
		validationErr = fmt.Errorf("Vault returned WrapInfo without WrappedAccessor. Secret warnings: %v", secret.Warnings)
	}
	if validationErr != nil {
		v.logger.Warn("failed to CreateToken", "error", validationErr)
		return nil, structs.NewRecoverableError(validationErr, true)
	}

	// Got a valid response
	return secret, nil
}

// LookupToken takes a Vault token and does a lookup against Vault. The call is
// rate limited and may be canceled with passed context.
func (v *vaultClient) LookupToken(ctx context.Context, token string) (*vapi.Secret, error) {
	if !v.Enabled() {
		return nil, fmt.Errorf("Vault integration disabled")
	}

	if !v.Active() {
		return nil, fmt.Errorf("Vault client not active")
	}

	// Check if we have established a connection with Vault
	if established, err := v.ConnectionEstablished(); !established && err == nil {
		return nil, structs.NewRecoverableError(fmt.Errorf("Connection to Vault has not been established"), true)
	} else if err != nil {
		return nil, err
	}

	// Track how long the request takes
	defer metrics.MeasureSince([]string{"nomad", "vault", "lookup_token"}, time.Now())

	// Ensure we are under our rate limit
	if err := v.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Lookup the token
	return v.auth.Lookup(token)
}

// RevokeTokens revokes the passed set of accessors. If committed is set, the
// purge function passed to the client is called. If there is an error purging
// either because of Vault failures or because of the purge function, the
// revocation is retried until the tokens TTL.
func (v *vaultClient) RevokeTokens(ctx context.Context, accessors []*structs.VaultAccessor, committed bool) error {
	if !v.Enabled() {
		return nil
	}

	if !v.Active() {
		return fmt.Errorf("Vault client not active")
	}

	// Track how long the request takes
	defer metrics.MeasureSince([]string{"nomad", "vault", "revoke_tokens"}, time.Now())

	// Check if we have established a connection with Vault. If not just add it
	// to the queue
	if established, err := v.ConnectionEstablished(); !established && err == nil {
		// Only bother tracking it for later revocation if the accessor was
		// committed
		if committed {
			v.storeForRevocation(accessors)
		}

		// Track that we are abandoning these accessors.
		metrics.IncrCounter([]string{"nomad", "vault", "undistributed_tokens_abandoned"}, float32(len(accessors)))
		return nil
	}

	// Attempt to revoke immediately and if it fails, add it to the revoke queue
	err := v.parallelRevoke(ctx, accessors)
	if err != nil {
		// If it is uncommitted, it is a best effort revoke as it will shortly
		// TTL within the cubbyhole and has not been leaked to any outside
		// system
		if !committed {
			metrics.IncrCounter([]string{"nomad", "vault", "undistributed_tokens_abandoned"}, float32(len(accessors)))
			return nil
		}

		v.logger.Warn("failed to revoke tokens. Will reattempt until TTL", "error", err)
		v.storeForRevocation(accessors)
		return nil
	} else if !committed {
		// Mark that it was revoked but there is nothing to purge so exit
		metrics.IncrCounter([]string{"nomad", "vault", "undistributed_tokens_revoked"}, float32(len(accessors)))
		return nil
	}

	if err := v.purgeFn(accessors); err != nil {
		v.logger.Error("failed to purge Vault accessors", "error", err)
		v.storeForRevocation(accessors)
		return nil
	}

	// Track that it was revoked successfully
	metrics.IncrCounter([]string{"nomad", "vault", "distributed_tokens_revoked"}, float32(len(accessors)))

	return nil
}

func (v *vaultClient) MarkForRevocation(accessors []*structs.VaultAccessor) error {
	if !v.Enabled() {
		return nil
	}

	if !v.Active() {
		return fmt.Errorf("Vault client not active")
	}

	v.storeForRevocation(accessors)
	return nil
}

// storeForRevocation stores the passed set of accessors for revocation. It
// captures their effective TTL by storing their create TTL plus the current
// time.
func (v *vaultClient) storeForRevocation(accessors []*structs.VaultAccessor) {
	v.revLock.Lock()

	now := time.Now()
	for _, a := range accessors {
		if _, ok := v.revoking[a]; !ok {
			v.revoking[a] = now.Add(time.Duration(a.CreationTTL) * time.Second)
		}
	}
	v.revLock.Unlock()
}

// parallelRevoke revokes the passed VaultAccessors in parallel.
func (v *vaultClient) parallelRevoke(ctx context.Context, accessors []*structs.VaultAccessor) error {
	if !v.Enabled() {
		return fmt.Errorf("Vault integration disabled")
	}

	if !v.Active() {
		return fmt.Errorf("Vault client not active")
	}

	// Check if we have established a connection with Vault
	if established, err := v.ConnectionEstablished(); !established && err == nil {
		return structs.NewRecoverableError(fmt.Errorf("Connection to Vault has not been established"), true)
	} else if err != nil {
		return err
	}

	g, pCtx := errgroup.WithContext(ctx)

	// Cap the handlers
	handlers := len(accessors)
	if handlers > maxParallelRevokes {
		handlers = maxParallelRevokes
	}

	// Revoke the Vault Token Accessors
	input := make(chan *structs.VaultAccessor, handlers)
	for i := 0; i < handlers; i++ {
		g.Go(func() error {
			for {
				select {
				case va, ok := <-input:
					if !ok {
						return nil
					}

					err := v.auth.RevokeAccessor(va.Accessor)
					if err != nil && !strings.Contains(err.Error(), "invalid accessor") {
						return fmt.Errorf("failed to revoke token (alloc: %q, node: %q, task: %q): %v", va.AllocID, va.NodeID, va.Task, err)
					}
				case <-pCtx.Done():
					return nil
				}
			}
		})
	}

	// Send the input
	go func() {
		defer close(input)
		for _, va := range accessors {
			select {
			case <-pCtx.Done():
				return
			case input <- va:
			}
		}

	}()

	// Wait for everything to complete
	return g.Wait()
}

// maxVaultRevokeBatchSize is the maximum tokens a revokeDaemon should revoke
// and purge at any given time.
//
// Limiting the revocation batch size is beneficial for few reasons:
//   - A single revocation failure of any entry in batch result into retrying the whole batch;
//     the larger the batch is the higher likelihood of such failure
//   - Smaller batch sizes result into more co-operativeness: provides hooks for
//     reconsidering token TTL and leadership steps down.
//   - Batches limit the size of the Raft message purging tokens. Due to bugs
//     pre-0.11.3, expired tokens were not properly purged, so users upgrading from
//     older versions may have huge numbers (millions) of expired tokens to purge.
const maxVaultRevokeBatchSize = 1000

// revokeDaemon should be called in a goroutine and is used to periodically
// revoke Vault accessors that failed the original revocation
func (v *vaultClient) revokeDaemon() {
	ticker := time.NewTicker(v.revocationIntv)
	defer ticker.Stop()

	for {
		select {
		case <-v.tomb.Dying():
			return
		case now := <-ticker.C:
			if established, err := v.ConnectionEstablished(); !established || err != nil {
				continue
			}

			v.revLock.Lock()

			// Fast path
			if len(v.revoking) == 0 {
				v.revLock.Unlock()
				continue
			}

			// Build the list of accessors that need to be revoked while pruning any TTL'd checks
			toRevoke := len(v.revoking)
			if toRevoke > v.maxRevokeBatchSize {
				v.logger.Info("batching tokens to be revoked",
					"to_revoke", toRevoke, "batch_size", v.maxRevokeBatchSize,
					"batch_interval", v.revocationIntv)
				toRevoke = v.maxRevokeBatchSize
			}
			revoking := make([]*structs.VaultAccessor, 0, toRevoke)
			ttlExpired := []*structs.VaultAccessor{}
			for va, ttl := range v.revoking {
				if now.After(ttl) {
					ttlExpired = append(ttlExpired, va)
				} else {
					revoking = append(revoking, va)
				}

				// Batches should consider tokens to be revoked
				// as well as expired tokens to ensure the Raft
				// message is reasonably sized.
				if len(revoking)+len(ttlExpired) >= toRevoke {
					break
				}
			}

			if err := v.parallelRevoke(context.Background(), revoking); err != nil {
				v.logger.Warn("background token revocation errored", "error", err)
				v.revLock.Unlock()
				continue
			}

			// Unlock before a potentially expensive operation
			v.revLock.Unlock()

			// purge all explicitly revoked as well as ttl expired tokens
			// and only remove them locally on purge success
			revoking = append(revoking, ttlExpired...)

			// Call the passed in token revocation function
			if err := v.purgeFn(revoking); err != nil {
				// Can continue since revocation is idempotent
				v.logger.Error("token revocation errored", "error", err)
				continue
			}

			// Track that tokens were revoked successfully
			metrics.IncrCounter([]string{"nomad", "vault", "distributed_tokens_revoked"}, float32(len(revoking)))

			// Can delete from the tracked list now that we have purged
			v.revLock.Lock()
			for _, va := range revoking {
				delete(v.revoking, va)
			}
			v.revLock.Unlock()

		}
	}
}

// purgeVaultAccessors creates a Raft transaction to remove the passed Vault
// Accessors
func (s *Server) purgeVaultAccessors(accessors []*structs.VaultAccessor) error {
	// Commit this update via Raft
	req := structs.VaultAccessorsRequest{Accessors: accessors}
	_, _, err := s.raftApply(structs.VaultAccessorDeregisterRequestType, req)
	return err
}

// wrapNilError is a helper that returns a wrapped function that returns a nil
// error
func wrapNilError(f func()) func() error {
	return func() error {
		f()
		return nil
	}
}

// setLimit is used to update the rate limit
func (v *vaultClient) setLimit(l rate.Limit) {
	v.l.Lock()
	defer v.l.Unlock()
	v.limiter = rate.NewLimiter(l, int(l))
}

func (v *vaultClient) Stats() map[string]string {
	stat := v.stats()

	expireTimeStr := ""
	if !stat.TokenExpiry.IsZero() {
		expireTimeStr = stat.TokenExpiry.Format(time.RFC3339)
	}

	lastRenewTimeStr := ""
	if !stat.LastRenewalTime.IsZero() {
		lastRenewTimeStr = stat.LastRenewalTime.Format(time.RFC3339)
	}

	nextRenewTimeStr := ""
	if !stat.NextRenewalTime.IsZero() {
		nextRenewTimeStr = stat.NextRenewalTime.Format(time.RFC3339)
	}

	return map[string]string{
		"tracked_for_revoked":     strconv.Itoa(stat.TrackedForRevoke),
		"token_ttl":               stat.TokenTTL.Round(time.Second).String(),
		"token_expire_time":       expireTimeStr,
		"token_last_renewal_time": lastRenewTimeStr,
		"token_next_renewal_time": nextRenewTimeStr,
	}
}

func (v *vaultClient) stats() *VaultStats {
	// Allocate a new stats struct
	stats := new(VaultStats)

	v.revLock.Lock()
	stats.TrackedForRevoke = len(v.revoking)
	v.revLock.Unlock()

	v.currentExpirationLock.Lock()
	stats.TokenExpiry = v.currentExpiration
	v.currentExpirationLock.Unlock()

	v.renewalTimeLock.Lock()
	stats.NextRenewalTime = v.nextRenewalTime
	stats.LastRenewalTime = v.lastRenewalTime
	v.renewalTimeLock.Unlock()

	if !stats.TokenExpiry.IsZero() {
		stats.TokenTTL = time.Until(stats.TokenExpiry)
	}

	if !stats.LastRenewalTime.IsZero() {
		stats.TimeFromLastRenewal = time.Since(stats.LastRenewalTime)
	}
	if !stats.NextRenewalTime.IsZero() {
		stats.TimeToNextRenewal = time.Until(stats.NextRenewalTime)
	}

	return stats
}

// EmitStats is used to export metrics about the blocked eval tracker while enabled
func (v *vaultClient) EmitStats(period time.Duration, stopCh <-chan struct{}) {
	timer, stop := helper.NewSafeTimer(period)
	defer stop()

	for {
		timer.Reset(period)

		select {
		case <-timer.C:
			stats := v.stats()
			metrics.SetGauge([]string{"nomad", "vault", "distributed_tokens_revoking"}, float32(stats.TrackedForRevoke))
			metrics.SetGauge([]string{"nomad", "vault", "token_ttl"}, float32(stats.TokenTTL/time.Millisecond))
			metrics.SetGauge([]string{"nomad", "vault", "token_last_renewal"}, float32(stats.TimeFromLastRenewal/time.Millisecond))
			metrics.SetGauge([]string{"nomad", "vault", "token_next_renewal"}, float32(stats.TimeToNextRenewal/time.Millisecond))

		case <-stopCh:
			return
		}
	}
}

// extendExpiration sets the current auth token expiration record to ttLSeconds seconds from now
func (v *vaultClient) extendExpiration(ttlSeconds int) {
	v.currentExpirationLock.Lock()
	v.currentExpiration = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	v.currentExpirationLock.Unlock()
}

// VaultVaultNoopDelegate returns the default vault api auth token handler
type VaultNoopDelegate struct{}

func (e *VaultNoopDelegate) clientForTask(v *vaultClient, namespace string) (*vapi.Client, error) {
	return v.client, nil
}
