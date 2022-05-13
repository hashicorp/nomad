package nomad

import (
	"bytes"
	"encoding/gob"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// SecureVariables endpoint serves RPCs for storing and retrieving
// encrypted variables
type SecureVariables struct {
	srv       *Server
	logger    hclog.Logger
	encrypter *Encrypter
}

func (sv *SecureVariables) Create(args *structs.SecureVariablesUpsertRequest, reply *structs.SecureVariablesUpsertResponse) error {
	if done, err := sv.srv.forward("SecureVariables.Create", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "create"}, time.Now())

	// TODO: implement real ACL checks
	if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	sv.logger.Trace("TODO") // silences structcheck lint

	// TODO: placeholder for serialization and encryption
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(args.Data.UnencryptedData)
	if err != nil {
		return err
	}

	args.Data.EncryptedData = &structs.SecureVariableData{}
	args.Data.EncryptedData.KeyID = "TODO"
	args.Data.EncryptedData.Data = sv.encrypter.Encrypt(buf.Bytes(), args.Data.EncryptedData.KeyID)

	// TODO: implementation
	SV_Upsert(args, reply)

	return nil
}

func (sv *SecureVariables) List(args *structs.SecureVariablesListRequest, reply *structs.SecureVariablesListResponse) error {
	if done, err := sv.srv.forward("SecureVariables.List", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "list"}, time.Now())

	// TODO: implement real ACL checks
	if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation
	SV_List(args, reply)

	return nil
}

func (sv *SecureVariables) Read(args *structs.SecureVariablesReadRequest, reply *structs.SecureVariablesReadResponse) error {
	if done, err := sv.srv.forward("SecureVariables.Read", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "read"}, time.Now())

	// TODO: implement real ACL checks
	if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation
	SV_Read(args, reply)

	return nil
}

func (sv *SecureVariables) Update(args *structs.SecureVariablesUpsertRequest, reply *structs.SecureVariablesUpsertResponse) error {
	if done, err := sv.srv.forward("SecureVariables.Update", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "update"}, time.Now())

	// TODO: implement real ACL checks
	if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation
	SV_Upsert(args, reply)

	return nil
}

func (sv *SecureVariables) Delete(args *structs.SecureVariablesDeleteRequest, reply *structs.SecureVariablesDeleteResponse) error {
	if done, err := sv.srv.forward("SecureVariables.Delete", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "delete"}, time.Now())

	// TODO: implement real ACL checks
	if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// TODO: implementation
	SV_Delete(args, reply)

	return nil
}
