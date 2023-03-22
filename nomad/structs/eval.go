package structs

const (
	// EvalDeleteRPCMethod is the RPC method for batch deleting evaluations
	// using their IDs.
	//
	// Args: EvalDeleteRequest
	// Reply: EvalDeleteResponse
	EvalDeleteRPCMethod = "Eval.Delete"
)

// EvalDeleteRequest is the request object used when operators are manually
// deleting evaluations. The number of evaluation IDs within the request must
// not be greater than MaxEvalIDsPerDeleteRequest.
type EvalDeleteRequest struct {
	EvalIDs []string
	WriteRequest
}

// EvalDeleteResponse is the response object when one or more evaluation are
// deleted manually by an operator.
type EvalDeleteResponse struct {
	WriteMeta
}
