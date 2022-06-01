package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Keyring endpoint serves RPCs for secure variables key management
type Keyring struct {
	srv       *Server
	logger    hclog.Logger
	encrypter *Encrypter
	ctx       *RPCContext // context for connection, to check TLS role
}

func (k *Keyring) Rotate(args *structs.KeyringRotateRootKeyRequest, reply *structs.KeyringRotateRootKeyResponse) error {
	if done, err := k.srv.forward("Keyring.Rotate", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "rotate"}, time.Now())

	if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	if args.Full {
		// TODO: implement full key rotation via a core job
	}
	if args.Algorithm == "" {
		args.Algorithm = structs.EncryptionAlgorithmAES256GCM
	}

	rootKey, err := structs.NewRootKey(args.Algorithm)
	if err != nil {
		return err
	}

	rootKey.Meta.Active = true

	// make sure it's been added to the local keystore before we write
	// it to raft, so that followers don't try to Get a key that
	// hasn't yet been written to disk
	err = k.encrypter.AddKey(rootKey)
	if err != nil {
		return err
	}

	// Update metadata via Raft so followers can retrieve this key
	req := structs.KeyringUpdateRootKeyMetaRequest{
		RootKeyMeta:  rootKey.Meta,
		WriteRequest: args.WriteRequest,
	}
	out, index, err := k.srv.raftApply(structs.RootKeyMetaUpsertRequestType, req)
	if err != nil {
		return err
	}
	if err, ok := out.(error); ok && err != nil {
		return err
	}
	reply.Key = rootKey.Meta
	reply.Index = index
	return nil
}

func (k *Keyring) List(args *structs.KeyringListRootKeyMetaRequest, reply *structs.KeyringListRootKeyMetaResponse) error {
	if done, err := k.srv.forward("Keyring.List", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "list"}, time.Now())

	// we need to allow both humans with management tokens and
	// non-leader servers to list keys, in order to support
	// replication
	err := validateTLSCertificateLevel(k.srv, k.ctx, tlsCertificateLevelServer)
	if err != nil {
		if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
			return err
		} else if aclObj != nil && !aclObj.IsManagement() {
			return structs.ErrPermissionDenied
		}
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {

			// retrieve all the key metadata
			snap, err := k.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			iter, err := snap.RootKeyMetas(ws)
			if err != nil {
				return err
			}

			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				keyMeta := raw.(*structs.RootKeyMeta)
				reply.Keys = append(reply.Keys, keyMeta)
			}
			return k.srv.replySetIndex(state.TableRootKeyMeta, &reply.QueryMeta)
		},
	}
	return k.srv.blockingRPC(&opts)
}

// Update updates an existing key in the keyring, including both the
// key material and metadata.
func (k *Keyring) Update(args *structs.KeyringUpdateRootKeyRequest, reply *structs.KeyringUpdateRootKeyResponse) error {
	if done, err := k.srv.forward("Keyring.Update", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "update"}, time.Now())

	if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	err := k.validateUpdate(args)
	if err != nil {
		return err
	}

	// make sure it's been added to the local keystore before we write
	// it to raft, so that followers don't try to Get a key that
	// hasn't yet been written to disk
	err = k.encrypter.AddKey(args.RootKey)
	if err != nil {
		return err
	}

	// unwrap the request to turn it into a meta update only
	metaReq := &structs.KeyringUpdateRootKeyMetaRequest{
		RootKeyMeta:  args.RootKey.Meta,
		WriteRequest: args.WriteRequest,
	}

	// update the metadata via Raft
	out, index, err := k.srv.raftApply(structs.RootKeyMetaUpsertRequestType, metaReq)
	if err != nil {
		return err
	}
	if err, ok := out.(error); ok && err != nil {
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
	keyMeta, err := snap.RootKeyMetaByID(ws, args.RootKey.Meta.KeyID)
	if err != nil {
		return err
	}
	if keyMeta != nil && keyMeta.Algorithm != args.RootKey.Meta.Algorithm {
		return fmt.Errorf("root key algorithm cannot be changed after a key is created")
	}

	return nil
}

// Get retrieves an existing key from the keyring, including both the
// key material and metadata. It is used only for replication.
func (k *Keyring) Get(args *structs.KeyringGetRootKeyRequest, reply *structs.KeyringGetRootKeyResponse) error {
	// ensure that only another server can make this request
	err := validateTLSCertificateLevel(k.srv, k.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
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

			// retrieve the key metadata
			snap, err := k.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			keyMeta, err := snap.RootKeyMetaByID(ws, args.KeyID)
			if err != nil {
				return err
			}
			if keyMeta == nil {
				return k.srv.replySetIndex(state.TableRootKeyMeta, &reply.QueryMeta)
			}

			// retrieve the key material from the keyring
			key, err := k.encrypter.GetKey(keyMeta.KeyID)
			if err != nil {
				return err
			}
			rootKey := &structs.RootKey{
				Meta: keyMeta,
				Key:  key,
			}
			reply.Key = rootKey
			reply.Index = keyMeta.ModifyIndex
			return nil
		},
	}
	return k.srv.blockingRPC(&opts)
}

func (k *Keyring) Delete(args *structs.KeyringDeleteRootKeyRequest, reply *structs.KeyringDeleteRootKeyResponse) error {
	if done, err := k.srv.forward("Keyring.Delete", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "delete"}, time.Now())

	if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	if args.KeyID == "" {
		return fmt.Errorf("root key ID is required")
	}

	// lookup any existing key and validate the delete
	snap, err := k.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	keyMeta, err := snap.RootKeyMetaByID(ws, args.KeyID)
	if err != nil {
		return err
	}
	if keyMeta == nil {
		return nil // safe to bail out early
	}
	if keyMeta.Active {
		return fmt.Errorf("active root key cannot be deleted - call rotate first")
	}

	// update via Raft
	out, index, err := k.srv.raftApply(structs.RootKeyMetaDeleteRequestType, args)
	if err != nil {
		return err
	}
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// remove the key from the keyring too
	k.encrypter.RemoveKey(args.KeyID)

	reply.Index = index
	return nil
}
