// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package keymgr

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/joseutil"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	ErrKeyNotFound = errors.New("public key not found")
)

type RPCer interface {
	RPC(method string, args any, reply any) error
}

type PubKeyCacheConfig struct {
	Region     string
	RPC        RPCer
	ShutdownCh <-chan struct{}
	Logger     hclog.Logger
}

// PubKeyCache is a cache for workload identity signing keys.
//
// This implementation relies on Run running in a goroutine for fetching the latest keys.
type PubKeyCache struct {
	// keys maps key ids to their public key
	keys atomic.Pointer[map[string]*structs.KeyringPublicKey]

	region     string
	rpc        RPCer
	shutdownCh <-chan struct{}
	firstRun   chan struct{} // closed after first keys fetched
	logger     hclog.Logger
}

func NewPubKeyCache(config PubKeyCacheConfig) *PubKeyCache {
	return &PubKeyCache{
		region:     config.Region,
		rpc:        config.RPC,
		shutdownCh: config.ShutdownCh,
		firstRun:   make(chan struct{}),
		logger:     config.Logger.Named("key_cache"),
	}
}

// Run fetches public keys and closes the firstRun channel after initial fetch.
func (c *PubKeyCache) Run() {
	timer, timerStop := helper.NewStoppedTimer()
	defer timerStop()

	args := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:     c.region,
			AllowStale: true,
		},
	}

	for {
		var rpcReply structs.KeyringListPublicResponse
		if err := c.rpc.RPC("Keyring.ListPublic", &args, &rpcReply); err != nil {
			// Wait and retry
			//TODO standardize somewhere? base it off something meaningful?!
			const base = 1 * time.Minute
			const jitter = 4 * time.Minute
			wait := base + helper.RandomStagger(jitter)
			c.logger.Error("error fetching public keys", "error", err, "retry_in", wait)
			timer.Reset(wait)
			select {
			case <-c.shutdownCh:
				return
			case <-timer.C:
				continue
			}
		}

		// Make sure we wait until keys have changed to process a response
		if rpcReply.Index > args.QueryOptions.MinQueryIndex {
			args.QueryOptions.MinQueryIndex = rpcReply.Index
		}

		// Build new key cache and track max create time for determining wait
		var maxCreate int64
		keys := make(map[string]*structs.KeyringPublicKey, len(rpcReply.PublicKeys))
		for _, pubKey := range rpcReply.PublicKeys {
			keys[pubKey.KeyID] = pubKey
			if pubKey.CreateTime > maxCreate {
				maxCreate = pubKey.CreateTime
			}
		}

		// Atomically overwrite keys
		c.keys.Store(&keys)

		// Ensure firstRun is closed after successfully loading keys
		select {
		case <-c.firstRun:
			// Already closed
		default:
			close(c.firstRun)
		}

		// Since keys may not change for days, sleep instead of trying to sit in
		// blocking queries for the entire time.
		left := time.Unix(0, maxCreate).Add(rpcReply.RotationThreshold).Sub(time.Now())

		// Wait 1/2 + rand(1/10) the time until the deadline.
		wait := (left / 2) + helper.RandomStagger(left/10)

		const minWait = 10 * time.Second //TODO(schmichael) is this long enough?
		if wait < minWait {
			wait = minWait + helper.RandomStagger(minWait)
		}

		timer.Reset(wait)

		select {
		case <-c.shutdownCh:
			return
		case <-timer.C:
		}
	}
}

// ParseJWT parses and validates a workload identity JWT. IdentityClaims are
// returned if the JWT is valid, otherwise an error.
//
// Does *not* assert that the workload is still running and therefore Nomad
// authn may still reject the token.
//
// Will return a context error if agent shutdown or context is canceled.
func (c *PubKeyCache) ParseJWT(ctx context.Context, raw string) (*structs.IdentityClaims, error) {
	token, err := jwt.ParseSigned(raw)
	if err != nil {
		return nil, err
	}

	keyID, err := joseutil.KeyID(token)
	if err != nil {
		return nil, err
	}

	pubKey, err := c.getPubKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	ic := structs.IdentityClaims{}
	if err := token.Claims(pubKey.PublicKey, &ic); err != nil {
		return nil, err
	}

	return &ic, nil
}

func (c *PubKeyCache) getPubKey(ctx context.Context, keyID string) (*structs.KeyringPublicKey, error) {
	select {
	case <-c.shutdownCh:
		// Since PubKeyCache is created in the Agent, and the Agent only has a
		// shutdown channel, not a shutdown context, we need to fake the context
		// error here.
		return nil, context.Canceled
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.firstRun:
	}

	keys := *c.keys.Load()
	pubKey := keys[keyID]
	if pubKey == nil {
		// We could force a refresh to ensure we're not missing an otherwise valid
		// key, but that would allow invalid tokens to cause the cluster to do a
		// lot more work than if we exit immediately here.
		return nil, ErrKeyNotFound
	}

	return pubKey, nil
}
