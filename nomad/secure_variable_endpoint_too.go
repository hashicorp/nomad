package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

func svePreApply(sv *SecureVariables, args *structs.SVRequest, vd *structs.SecureVariableDecrypted) (ok bool, canRead bool, err error) {

	ok = false
	canRead = false
	var aclObj *acl.ACL

	// Perform the ACL token resolution.
	if aclObj, err = sv.srv.ResolveToken(args.AuthToken); err != nil {
		return
	} else if aclObj != nil {
		hasPerm := func(perm string) bool {
			return aclObj.AllowSecureVariableOperation(args.Var.Namespace,
				args.Var.Path, perm)
		}
		canRead = hasPerm(acl.SecureVariablesCapabilityRead)

		switch args.Op {
		case structs.SVSet:
			if !hasPerm(acl.SecureVariablesCapabilityWrite) {
				err = structs.ErrPermissionDenied
				return
			}

		case structs.SVCAS:
			if !hasPerm(acl.SecureVariablesCapabilityWrite) {
				err = structs.ErrPermissionDenied
				return
			}

		case structs.SVDelete:
			if !hasPerm(acl.SecureVariablesCapabilityDestroy) {
				err = structs.ErrPermissionDenied
				return
			}

		case structs.SVDeleteCAS:
			if !hasPerm(acl.SecureVariablesCapabilityDestroy) {
				err = structs.ErrPermissionDenied
				return
			}

		default:
			err = fmt.Errorf("svPreApply: unexpected SVOp received: %q", args.Op)
			return
		}
	} else {
		// ACLs are not enabled.
		canRead = true
	}

	if err = args.Var.Validate(); err != nil {
		return
	}

	return
}

// Apply is used to apply a SV update request to the data store.
func (sv *SecureVariables) Apply(args *structs.SVRequest, reply *structs.SVResponse) error {
	if done, err := sv.srv.forward(structs.SecureVariablesApplyRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "apply"}, time.Now())

	// TODO: what to do with Ok, delete later if need be
	_, canRead, err := svePreApply(sv, args, args.Var)
	if err != nil {
		return err
	}

	// TODO: Maybe do a CAS check here?

	// Encrypt
	ev, err := sv.encrypt(args.Var)
	if err != nil {
		return fmt.Errorf("secure variable error: encrypt: %w", err)
	}

	// Make a SVEArgs
	sveArgs := structs.SVERequest{
		Op:           args.Op,
		Var:          ev,
		WriteRequest: args.WriteRequest,
	}

	// Apply the update.
	out, index, err := sv.srv.raftApply(structs.SVERequestType, sveArgs)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}

	r, err := sv.makeSVResponse(args, out.(*structs.SVEResponse), canRead)
	if err != nil {
		return err
	}
	*reply = *r
	reply.Index = index
	return nil
}

// MakeSVResponse merges the output of this SVEResponse with the
// SecureVariableDataItems
func (sv *SecureVariables) makeSVResponse(
	req *structs.SVRequest, eResp *structs.SVEResponse,
	canRead bool) (*structs.SVResponse, error) {

	out := structs.SVResponse{
		Op:        eResp.Op,
		Input:     req.Var,
		Result:    eResp.Result,
		Error:     eResp.Error,
		WriteMeta: eResp.WriteMeta,
	}

	if eResp.IsOk() {
		// The writer is allowed to read their own write
		out.Output = &structs.SecureVariableDecrypted{
			SecureVariableMetadata: *eResp.WrittenSVMeta,
			Items:                  req.Var.Items.Copy(),
		}
		return &out, nil
	}

	// At this point, the response is necessarily a conflict.
	// Prime output from the encrypted responses metadata
	out.Conflict = &structs.SecureVariableDecrypted{
		SecureVariableMetadata: eResp.Conflict.SecureVariableMetadata,
		Items:                  nil,
	}

	// If the caller can't read the conflicting value, return the
	// metadata, but no items and flag it as redacted
	if !canRead {
		out.Result = structs.SVOpResultRedacted
		return &out, nil
	}

	// At this point, the caller has read access to the conflicting
	// value so we can return it in the output; decrypt it.
	dv, err := sv.decrypt(eResp.Conflict)
	if err != nil {
		return nil, err
	}

	// Store it in conflict and ship it
	out.Conflict = dv
	return &out, nil
}
