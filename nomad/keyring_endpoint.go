package nomad

import (
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/nomad/structs"
)

// KeyRing endpoint serves RPCs for secure variables key management
type KeyRing struct {
	srv       *Server
	logger    hclog.Logger
	encrypter *Encrypter
}

func (k *KeyRing) Rotate(args *structs.KeyringRotateRootKeyRequest, reply *structs.KeyringRotateRootKeyResponse) error {
	if done, err := k.srv.forward("KeyRing.Rotate", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "rotate"}, time.Now())

	// TODO: allow for servers to force rotation as well
	if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation; this just silences the structcheck lint
	for keyID := range k.encrypter.ciphers {
		k.logger.Trace("TODO", "key", keyID)
	}
	return nil
}

func (k *KeyRing) List(args *structs.KeyringListRootKeyMetaRequest, reply *structs.KeyringListRootKeyMetaResponse) error {
	if done, err := k.srv.forward("KeyRing.List", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "list"}, time.Now())

	// TODO: probably need to allow for servers to list keys as well, to support replication?
	if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation

	return nil
}

func (k *KeyRing) Update(args *structs.KeyringUpdateRootKeyRequest, reply *structs.KeyringUpdateRootKeyResponse) error {
	if done, err := k.srv.forward("KeyRing.Update", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "update"}, time.Now())

	// TODO: need to allow for servers to update keys as well, to support replication
	if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation

	return nil
}

func (k *KeyRing) Delete(args *structs.KeyringDeleteRootKeyRequest, reply *structs.KeyringDeleteRootKeyResponse) error {
	if done, err := k.srv.forward("KeyRing.Delete", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "keyring", "delete"}, time.Now())

	// TODO: need to allow for servers to delete keys as well, to support replication
	if aclObj, err := k.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation

	return nil
}
