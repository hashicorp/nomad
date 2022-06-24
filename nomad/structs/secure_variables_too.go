package structs

/*
	 _____________________________________________________
	|                                                     |
	|  This contains the ConsulKV Style Secure Variables  |
	|  API. It's split out so I don't lose my mind.       |
	|  -cv                                                |
	|_____________________________________________________|

*/

const SecureVariablesApplyRPCMethod = "SecureVariables.Apply"

// SVRequest is used by users to operate on the secure variable store
type SVRequest struct {
	Op  SVOp                     // Operation to be performed during apply
	Var *SecureVariableDecrypted // Variable-shaped request data
	WriteRequest
}

// SVResponse is sent back to the user to inform them of success or failure
type SVResponse struct {
	Op       SVOp                     // Operation performed
	Input    *SecureVariableDecrypted // Input supplied
	Result   SVOpResult               // Return status from operation
	Error    error                    // Error if any
	Conflict *SecureVariableDecrypted // Conflicting value if applicable
	Output   *SecureVariableDecrypted // Operation Result if successful; nil for successful deletes
	WriteMeta
}

// SVERequest is used by the FSM to modify the secure variable store
type SVERequest struct {
	Op  SVOp                     // Which operation are we performing
	Var *SecureVariableEncrypted // Which directory entry
	WriteRequest
}

// SVEResponse is used by the FSM to inform the RPC layer of success or failure
type SVEResponse struct {
	Op            SVOp                     // Which operation did we performing
	Result        SVOpResult               // What happened (ok, conflict, error)
	Error         error                    // error if any
	Conflict      *SecureVariableEncrypted // conflicting secure variable if applies
	WrittenSVMeta *SecureVariableMetadata  // for making the SVResponse
	WriteMeta
}

// SVOp constants give possible operations available in a transaction.
type SVOp string

const (
	SVSet       SVOp = "set"
	SVDelete    SVOp = "delete"
	SVDeleteCAS SVOp = "delete-cas"
	SVCAS       SVOp = "cas"
)

// SVOpResult constants give possible operations results from a transaction.
type SVOpResult string

const (
	SVOpResultOk       SVOpResult = "ok"
	SVOpResultConflict SVOpResult = "conflict"
	SVOpResultRedacted SVOpResult = "conflict-redacted"
	SVOpResultError    SVOpResult = "error"
)

func (r *SVERequest) ErrorResponse(raftIndex uint64, err error) *SVEResponse {
	return &SVEResponse{
		Op:        r.Op,
		Result:    SVOpResultError,
		Error:     err,
		WriteMeta: WriteMeta{Index: raftIndex},
	}
}

func (r *SVERequest) SuccessResponse(raftIndex uint64, meta *SecureVariableMetadata) *SVEResponse {
	return &SVEResponse{
		Op:            r.Op,
		Result:        SVOpResultOk,
		WrittenSVMeta: meta,
		WriteMeta:     WriteMeta{Index: raftIndex},
	}
}

func (r *SVERequest) ConflictResponse(raftIndex uint64, cv *SecureVariableEncrypted) *SVEResponse {
	var cvCopy SecureVariableEncrypted
	if cv != nil {
		// make a copy so that we aren't sending
		// the live state store version
		cvCopy = cv.Copy()
	}
	return &SVEResponse{
		Op:        r.Op,
		Result:    SVOpResultConflict,
		Conflict:  &cvCopy,
		WriteMeta: WriteMeta{Index: raftIndex},
	}
}

func (r *SVEResponse) IsOk() bool {
	return r.Result == SVOpResultOk
}

func (r *SVEResponse) IsConflict() bool {
	return r.Result == SVOpResultConflict
}

func (r *SVEResponse) IsError() bool {
	// FIXME: This is brittle and requires immense faith that
	// the response is properly managed.
	return r.Result == SVOpResultError
}

func (r *SVResponse) IsOk() bool {
	return r.Result == SVOpResultOk
}

func (r *SVResponse) IsConflict() bool {
	return r.Result == SVOpResultConflict || r.Result == SVOpResultRedacted
}

func (r *SVResponse) IsError() bool {
	return r.Result == SVOpResultError
}

func (r *SVResponse) IsRedacted() bool {
	return r.Result == SVOpResultRedacted
}
