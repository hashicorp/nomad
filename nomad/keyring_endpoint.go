// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Keyring endpoint serves RPCs for root key management
type Keyring struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger

	encrypter *Encrypter
}

func NewKeyringEndpoint(srv *Server, ctx *RPCContext, enc *Encrypter) *Keyring {
	return &Keyring{srv: srv, ctx: ctx, logger: srv.logger.Named("keyring"), encrypter: enc}
}

func (k *Keyring) Rotate(args *structs.KeyringRotateRootKeyRequest, reply *structs.KeyringRotateRootKeyResponse) error {

	authErr := k.srv.Authenticate(k.ctx, args)
	if done, err := k.srv.forward("Keyring.Rotate", args, args, reply); done {
		return err
	}
	k.srv.MeasureRPCRate("keyring", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "keyring", "rotate"}, time.Now())

	if aclObj, err := k.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	if args.Algorithm == "" {
		args.Algorithm = structs.EncryptionAlgorithmAES256GCM
	}
	if args.Full && args.PublishTime > 0 {
		return fmt.Errorf("keyring cannot be prepublished and full rotated at the same time")
	}

	unwrappedKey, err := structs.NewUnwrappedRootKey(args.Algorithm)
	if err != nil {
		return err
	}

	if args.PublishTime != 0 {
		unwrappedKey.Meta.State = structs.RootKeyStatePrepublished
		unwrappedKey.Meta.PublishTime = args.PublishTime
	} else {
		unwrappedKey.Meta.State = structs.RootKeyStateActive
	}

	isClusterUpgraded := ServersMeetMinimumVersion(
		k.srv.serf.Members(), k.srv.Region(), minVersionKeyringInRaft, true)

	// wrap/encrypt the key before we write it to Raft
	wrappedKey, err := k.encrypter.AddUnwrappedKey(unwrappedKey, isClusterUpgraded)
	if err != nil {
		return err
	}

	var index uint64
	if isClusterUpgraded {
		_, index, err = k.srv.raftApply(structs.WrappedRootKeysUpsertRequestType,
			structs.KeyringUpsertWrappedRootKeyRequest{
				WrappedRootKeys: wrappedKey,
				Rekey:           args.Full,
				WriteRequest:    args.WriteRequest,
			})
	} else {
		// COMPAT(1.12.0): remove the version check and this code path
		_, index, err = k.srv.raftApply(structs.RootKeyMetaUpsertRequestType,
			structs.KeyringUpdateRootKeyMetaRequest{
				RootKeyMeta:  wrappedKey.Meta(),
				Rekey:        args.Full,
				WriteRequest: args.WriteRequest,
			})
	}
	if err != nil {
		return err
	}

	reply.Key = unwrappedKey.Meta
	reply.Index = index

	if args.Full {
		// like most core jobs, we don't commit this to raft b/c it's not
		// going to be periodically recreated and the ACL is from this leader
		eval := &structs.Evaluation{
			ID:          uuid.Generate(),
			Namespace:   "-",
			Priority:    structs.CoreJobPriority,
			Type:        structs.JobTypeCore,
			TriggeredBy: structs.EvalTriggerJobRegister,
			JobID:       structs.CoreJobVariablesRekey,
			Status:      structs.EvalStatusPending,
			ModifyIndex: index,
			LeaderACL:   k.srv.getLeaderAcl(),
		}
		k.srv.evalBroker.Enqueue(eval)
	}

	return nil
}

func (k *Keyring) List(args *structs.KeyringListRootKeyMetaRequest, reply *structs.KeyringListRootKeyMetaResponse) error {

	authErr := k.srv.Authenticate(k.ctx, args)
	if done, err := k.srv.forward("Keyring.List", args, args, reply); done {
		return err
	}
	k.srv.MeasureRPCRate("keyring", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "list"}, time.Now())

	if aclObj, err := k.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			iter, err := store.RootKeys(ws)
			if err != nil {
				return err
			}
			keys := []*structs.RootKeyMeta{}
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				rootKey := raw.(*structs.RootKey)
				keys = append(keys, rootKey.Meta())
			}

			reply.Keys = keys
			return k.srv.replySetIndex(state.TableRootKeys, &reply.QueryMeta)
		},
	}
	return k.srv.blockingRPC(&opts)
}

// Update updates an existing key in the keyring, including both the
// key material and metadata.
func (k *Keyring) Update(args *structs.KeyringUpdateRootKeyRequest, reply *structs.KeyringUpdateRootKeyResponse) error {

	authErr := k.srv.Authenticate(k.ctx, args)
	if done, err := k.srv.forward("Keyring.Update", args, args, reply); done {
		return err
	}
	k.srv.MeasureRPCRate("keyring", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "update"}, time.Now())

	if aclObj, err := k.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	err := k.validateUpdate(args)
	if err != nil {
		return err
	}

	isClusterUpgraded := ServersMeetMinimumVersion(
		k.srv.serf.Members(), k.srv.Region(), minVersionKeyringInRaft, true)

	// make sure it's been added to the local keystore before we write
	// it to raft, so that followers don't try to Get a key that
	// hasn't yet been written to disk
	wrappedKey, err := k.encrypter.AddUnwrappedKey(args.RootKey, isClusterUpgraded)
	if err != nil {
		return err
	}

	var index uint64
	if isClusterUpgraded {
		_, index, err = k.srv.raftApply(structs.WrappedRootKeysUpsertRequestType,
			structs.KeyringUpsertWrappedRootKeyRequest{
				WrappedRootKeys: wrappedKey,
				WriteRequest:    args.WriteRequest,
			})
	} else {
		// COMPAT(1.12.0): remove the version check and this code path
		// unwrap the request to turn it into a meta update only
		metaReq := &structs.KeyringUpdateRootKeyMetaRequest{
			RootKeyMeta:  args.RootKey.Meta,
			WriteRequest: args.WriteRequest,
		}

		// update the metadata via Raft
		_, index, err = k.srv.raftApply(structs.RootKeyMetaUpsertRequestType, metaReq)
	}
	if err != nil {
		return err
	}

	reply.Index = index
	return nil
}

// validateUpdate validates both the request and that any change to an
// existing key is valid
func (k *Keyring) validateUpdate(args *structs.KeyringUpdateRootKeyRequest) error {

	err := args.RootKey.Meta.Validate()
	if err != nil {
		return err
	}
	if len(args.RootKey.Key) == 0 {
		return fmt.Errorf("root key material is required")
	}

	// lookup any existing key and validate the update
	snap, err := k.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	rootKey, err := snap.RootKeyByID(ws, args.RootKey.Meta.KeyID)
	if err != nil {
		return err
	}
	if rootKey != nil && rootKey.Algorithm != args.RootKey.Meta.Algorithm {
		return fmt.Errorf("root key algorithm cannot be changed after a key is created")
	}

	return nil
}

// Get retrieves an existing key from the keyring, including both the
// key material and metadata. It is used only for replication.
func (k *Keyring) Get(args *structs.KeyringGetRootKeyRequest, reply *structs.KeyringGetRootKeyResponse) error {
	aclObj, err := k.srv.AuthenticateServerOnly(k.ctx, args)
	k.srv.MeasureRPCRate("keyring", structs.RateMetricRead, args)

	if err != nil || !aclObj.AllowServerOp() {
		return structs.ErrPermissionDenied
	}

	if done, err := k.srv.forward("Keyring.Get", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "keyring", "get"}, time.Now())

	if args.KeyID == "" {
		return fmt.Errorf("root key ID is required")
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {

			snap, err := k.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			wrappedKey, err := snap.RootKeyByID(ws, args.KeyID)
			if err != nil {
				return err
			}
			if wrappedKey == nil {
				return k.srv.replySetIndex(state.TableRootKeys, &reply.QueryMeta)
			}

			// retrieve the key material from the keyring
			unwrappedKey, err := k.encrypter.GetKey(wrappedKey.KeyID)
			if err != nil {
				return err
			}
			reply.Key = unwrappedKey
			err = k.srv.replySetIndex(state.TableRootKeys, &reply.QueryMeta)
			if err != nil {
				return err
			}

			return nil
		},
	}
	return k.srv.blockingRPC(&opts)
}

func (k *Keyring) Delete(args *structs.KeyringDeleteRootKeyRequest, reply *structs.KeyringDeleteRootKeyResponse) error {

	authErr := k.srv.Authenticate(k.ctx, args)
	if done, err := k.srv.forward("Keyring.Delete", args, args, reply); done {
		return err
	}
	k.srv.MeasureRPCRate("keyring", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "delete"}, time.Now())

	if aclObj, err := k.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	if args.KeyID == "" {
		return fmt.Errorf("root key ID is required")
	}

	// lookup any existing key and validate the delete
	var index uint64
	snap, err := k.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	rootKey, err := snap.RootKeyByID(ws, args.KeyID)
	if err != nil {
		return err
	}
	if rootKey != nil && rootKey.IsActive() {
		return fmt.Errorf("active root key cannot be deleted - call rotate first")
	}

	_, index, err = k.srv.raftApply(structs.WrappedRootKeysDeleteRequestType, args)
	if err != nil {
		return err
	}

	// remove the key from the keyring too
	k.encrypter.RemoveKey(args.KeyID)

	reply.Index = index
	return nil
}

// ListPublic signing keys used for workload identities. This RPC is used to
// back a JWKS endpoint.
//
// Unauthenticated because public keys are not sensitive.
func (k *Keyring) ListPublic(args *structs.GenericRequest, reply *structs.KeyringListPublicResponse) error {

	// JWKS is a public endpoint: intentionally ignore auth errors and only
	// authenticate to measure rate metrics.
	k.srv.Authenticate(k.ctx, args)
	if done, err := k.srv.forward("Keyring.ListPublic", args, args, reply); done {
		return err
	}
	k.srv.MeasureRPCRate("keyring", structs.RateMetricList, args)

	defer metrics.MeasureSince([]string{"nomad", "keyring", "list_public"}, time.Now())

	// Expose root_key_rotation_threshold so consumers can determine reasonable
	// cache settings.
	reply.RotationThreshold = k.srv.config.RootKeyRotationThreshold

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			iter, err := store.RootKeys(ws)
			if err != nil {
				return err
			}
			pubKeys := []*structs.KeyringPublicKey{}
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				wrappedKeys := raw.(*structs.RootKey)
				if wrappedKeys.State == structs.RootKeyStateDeprecated {
					// Only include valid keys
					continue
				}

				pubKey, err := k.encrypter.GetPublicKey(wrappedKeys.KeyID)
				if err != nil {
					return err
				}

				pubKeys = append(pubKeys, pubKey)

			}
			reply.PublicKeys = pubKeys
			return k.srv.replySetIndex(state.TableRootKeys, &reply.QueryMeta)
		},
	}
	return k.srv.blockingRPC(&opts)
}

// GetConfig for workload identities. This RPC is used to back an OIDC
// Discovery endpoint.
//
// Unauthenticated because OIDC Discovery endpoints must be publically
// available.
func (k *Keyring) GetConfig(args *structs.GenericRequest, reply *structs.KeyringGetConfigResponse) error {

	// JWKS is a public endpoint: intentionally ignore auth errors and only
	// authenticate to measure rate metrics.
	k.srv.Authenticate(k.ctx, args)
	if done, err := k.srv.forward("Keyring.GetConfig", args, args, reply); done {
		return err
	}
	k.srv.MeasureRPCRate("keyring", structs.RateMetricList, args)

	defer metrics.MeasureSince([]string{"nomad", "keyring", "get_config"}, time.Now())

	reply.OIDCDiscovery = k.srv.oidcDisco
	return nil
}
