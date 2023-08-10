// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	errNoLeader                   = "No cluster leader"
	errNotReadyForConsistentReads = "Not ready to serve consistent reads"
	errNoRegionPath               = "No path to region"
	errTokenNotFound              = "ACL token not found"
	errTokenExpired               = "ACL token expired"
	errTokenInvalid               = "ACL token is invalid" // not a UUID
	errPermissionDenied           = "Permission denied"
	errJobRegistrationDisabled    = "Job registration, dispatch, and scale are disabled by the scheduler configuration"
	errNoNodeConn                 = "No path to node"
	errUnknownMethod              = "Unknown rpc method"
	errUnknownNomadVersion        = "Unable to determine Nomad version"
	errNodeLacksRpc               = "Node does not support RPC; requires 0.8 or later"
	errMissingAllocID             = "Missing allocation ID"
	errIncompatibleFiltering      = "Filter expression cannot be used with other filter parameters"
	errMalformedChooseParameter   = "Parameter for choose must be in form '<number>|<key>'"

	// Prefix based errors that are used to check if the error is of a given
	// type. These errors should be created with the associated constructor.
	ErrUnknownAllocationPrefix = "Unknown allocation"
	ErrUnknownNodePrefix       = "Unknown node"
	ErrUnknownJobPrefix        = "Unknown job"
	ErrUnknownEvaluationPrefix = "Unknown evaluation"
	ErrUnknownDeploymentPrefix = "Unknown deployment"

	errRPCCodedErrorPrefix = "RPC Error:: "

	errDeploymentTerminalNoCancel    = "can't cancel terminal deployment"
	errDeploymentTerminalNoFail      = "can't fail terminal deployment"
	errDeploymentTerminalNoPause     = "can't pause terminal deployment"
	errDeploymentTerminalNoPromote   = "can't promote terminal deployment"
	errDeploymentTerminalNoResume    = "can't resume terminal deployment"
	errDeploymentTerminalNoUnblock   = "can't unblock terminal deployment"
	errDeploymentTerminalNoRun       = "can't run terminal deployment"
	errDeploymentTerminalNoSetHealth = "can't set health of allocations for a terminal deployment"
	errDeploymentRunningNoUnblock    = "can't unblock running deployment"
)

var (
	ErrNoLeader                   = errors.New(errNoLeader)
	ErrNotReadyForConsistentReads = errors.New(errNotReadyForConsistentReads)
	ErrNoRegionPath               = errors.New(errNoRegionPath)
	ErrTokenNotFound              = errors.New(errTokenNotFound)
	ErrTokenExpired               = errors.New(errTokenExpired)
	ErrTokenInvalid               = errors.New(errTokenInvalid)
	ErrPermissionDenied           = errors.New(errPermissionDenied)
	ErrJobRegistrationDisabled    = errors.New(errJobRegistrationDisabled)
	ErrNoNodeConn                 = errors.New(errNoNodeConn)
	ErrUnknownMethod              = errors.New(errUnknownMethod)
	ErrUnknownNomadVersion        = errors.New(errUnknownNomadVersion)
	ErrNodeLacksRpc               = errors.New(errNodeLacksRpc)
	ErrMissingAllocID             = errors.New(errMissingAllocID)
	ErrIncompatibleFiltering      = errors.New(errIncompatibleFiltering)
	ErrMalformedChooseParameter   = errors.New(errMalformedChooseParameter)

	ErrUnknownNode = errors.New(ErrUnknownNodePrefix)

	ErrDeploymentTerminalNoCancel    = errors.New(errDeploymentTerminalNoCancel)
	ErrDeploymentTerminalNoFail      = errors.New(errDeploymentTerminalNoFail)
	ErrDeploymentTerminalNoPause     = errors.New(errDeploymentTerminalNoPause)
	ErrDeploymentTerminalNoPromote   = errors.New(errDeploymentTerminalNoPromote)
	ErrDeploymentTerminalNoResume    = errors.New(errDeploymentTerminalNoResume)
	ErrDeploymentTerminalNoUnblock   = errors.New(errDeploymentTerminalNoUnblock)
	ErrDeploymentTerminalNoRun       = errors.New(errDeploymentTerminalNoRun)
	ErrDeploymentTerminalNoSetHealth = errors.New(errDeploymentTerminalNoSetHealth)
	ErrDeploymentRunningNoUnblock    = errors.New(errDeploymentRunningNoUnblock)

	ErrCSIClientRPCIgnorable  = errors.New("CSI client error (ignorable)")
	ErrCSIClientRPCRetryable  = errors.New("CSI client error (retryable)")
	ErrCSIVolumeMaxClaims     = errors.New("volume max claims reached")
	ErrCSIVolumeUnschedulable = errors.New("volume is currently unschedulable")
)

// IsErrNoLeader returns whether the error is due to there being no leader.
func IsErrNoLeader(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNoLeader)
}

// IsErrNoRegionPath returns whether the error is due to there being no path to
// the given region.
func IsErrNoRegionPath(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNoRegionPath)
}

// IsErrTokenNotFound returns whether the error is due to the passed token not
// being resolvable.
func IsErrTokenNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), errTokenNotFound)
}

// IsErrPermissionDenied returns whether the error is due to the operation not
// being allowed due to lack of permissions.
func IsErrPermissionDenied(err error) bool {
	return err != nil && strings.Contains(err.Error(), errPermissionDenied)
}

// IsErrNoNodeConn returns whether the error is due to there being no path to
// the given node.
func IsErrNoNodeConn(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNoNodeConn)
}

// IsErrUnknownMethod returns whether the error is due to the operation not
// being allowed due to lack of permissions.
func IsErrUnknownMethod(err error) bool {
	return err != nil && strings.Contains(err.Error(), errUnknownMethod)
}

func IsErrRPCCoded(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), errRPCCodedErrorPrefix)
}

// NewErrUnknownAllocation returns a new error caused by the allocation being
// unknown.
func NewErrUnknownAllocation(allocID string) error {
	return fmt.Errorf("%s %q", ErrUnknownAllocationPrefix, allocID)
}

// NewErrUnknownNode returns a new error caused by the node being unknown.
func NewErrUnknownNode(nodeID string) error {
	return fmt.Errorf("%s %q", ErrUnknownNodePrefix, nodeID)
}

// NewErrUnknownJob returns a new error caused by the job being unknown.
func NewErrUnknownJob(jobID string) error {
	return fmt.Errorf("%s %q", ErrUnknownJobPrefix, jobID)
}

// NewErrUnknownEvaluation returns a new error caused by the evaluation being
// unknown.
func NewErrUnknownEvaluation(evaluationID string) error {
	return fmt.Errorf("%s %q", ErrUnknownEvaluationPrefix, evaluationID)
}

// NewErrUnknownDeployment returns a new error caused by the deployment being
// unknown.
func NewErrUnknownDeployment(deploymentID string) error {
	return fmt.Errorf("%s %q", ErrUnknownDeploymentPrefix, deploymentID)
}

// IsErrUnknownAllocation returns whether the error is due to an unknown
// allocation.
func IsErrUnknownAllocation(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrUnknownAllocationPrefix)
}

// IsErrUnknownNode returns whether the error is due to an unknown
// node.
func IsErrUnknownNode(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrUnknownNodePrefix)
}

// IsErrUnknownJob returns whether the error is due to an unknown
// job.
func IsErrUnknownJob(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrUnknownJobPrefix)
}

// IsErrUnknownEvaluation returns whether the error is due to an unknown
// evaluation.
func IsErrUnknownEvaluation(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrUnknownEvaluationPrefix)
}

// IsErrUnknownDeployment returns whether the error is due to an unknown
// deployment.
func IsErrUnknownDeployment(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrUnknownDeploymentPrefix)
}

// IsErrUnknownNomadVersion returns whether the error is due to Nomad being
// unable to determine the version of a node.
func IsErrUnknownNomadVersion(err error) bool {
	return err != nil && strings.Contains(err.Error(), errUnknownNomadVersion)
}

// IsErrNodeLacksRpc returns whether error is due to a Nomad server being
// unable to connect to a client node because the client is too old (pre-v0.8).
func IsErrNodeLacksRpc(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNodeLacksRpc)
}

func IsErrNoSuchFileOrDirectory(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such file or directory")
}

// NewErrRPCCoded wraps an RPC error with a code to be converted to HTTP status
// code
func NewErrRPCCoded(code int, msg string) error {
	return fmt.Errorf("%s%d,%s", errRPCCodedErrorPrefix, code, msg)
}

// NewErrRPCCodedf wraps an RPC error with a code to be converted to HTTP
// status code.
func NewErrRPCCodedf(code int, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s%d,%s", errRPCCodedErrorPrefix, code, msg)
}

// CodeFromRPCCodedErr returns the code and message of error if it's an RPC error
// created through NewErrRPCCoded function.  Returns `ok` false if error is not
// an rpc error
func CodeFromRPCCodedErr(err error) (code int, msg string, ok bool) {
	if err == nil || !strings.HasPrefix(err.Error(), errRPCCodedErrorPrefix) {
		return 0, "", false
	}

	headerLen := len(errRPCCodedErrorPrefix)
	parts := strings.SplitN(err.Error()[headerLen:], ",", 2)
	if len(parts) != 2 {
		return 0, "", false
	}

	code, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}

	return code, parts[1], true
}
