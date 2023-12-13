// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	// Filter specifies the go-bexpr filter expression to be used for deleting a
	// set of evaluations that matches the filter
	Filter string

	WriteRequest
}

// EvalDeleteResponse is the response object when one or more evaluation are
// deleted manually by an operator.
type EvalDeleteResponse struct {
	Count int // how many Evaluations were safe to delete and/or matched the filter
	WriteMeta
}
