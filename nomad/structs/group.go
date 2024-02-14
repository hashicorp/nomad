// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/pointer"
)

const (
	// ReconcileOption is used to specify the behavior of the reconciliation process
	// between the original allocations and the replacements when a previously
	// disconnected client comes back online.
	ReconcileOptionKeepOriginal    = "keep_original"
	ReconcileOptionKeepReplacement = "keep_replacement"
	ReconcileOptionBestScore       = "best_score"
	ReconcileOptionLongestRunning  = "longest_running"
)

var (
	// Disconnect strategy validation errors
	errStopAndLost         = errors.New("Disconnect cannot be configured with both lost_after and stop_after")
	errNegativeLostAfter   = errors.New("lost_after cannot be a negative duration")
	errNegativeStopAfter   = errors.New("stop_after cannot be a negative duration")
	errStopAfterNonService = errors.New("stop_after can only be used with service or batch job types")
	errInvalidReconcile    = errors.New("reconcile option is invalid")
)

func NewDefaultDisconnectStrategy() *DisconnectStrategy {
	return &DisconnectStrategy{
		Replace:   pointer.Of(true),
		Reconcile: ReconcileOptionBestScore,
	}
}

// Disconnect strategy defines how both clients and server should behave in case of
// disconnection between them.
type DisconnectStrategy struct {
	// Defines for how long the server will consider the unresponsive node as
	// disconnected but alive instead of lost.
	LostAfter time.Duration `mapstructure:"lost_after" hcl:"lost_after,optional"`

	// Defines for how long a disconnected client will keep its allocations running.
	// This option has a different behavior for nil, the default, and time.Duration(0),
	// and needs to be intentionally set/unset.
	StopOnClientAfter *time.Duration `mapstructure:"stop_on_client_after" hcl:"stop_on_client_after, optional"`

	// A boolean field used to define if the allocations should be replaced while
	// its  considered disconnected.
	// This option has a different behavior for nil, the default, and false,
	// and needs to be intentionally set/unset. It needs to be set to true
	// for compatibility.
	Replace *bool `mapstructure:"replace" hcl:"replace,optional"`

	// Once the disconnected node starts reporting again, it will define which
	// instances to keep: the original allocations, the replacement, the one
	// running on the node with the best score as it is currently implemented,
	// or the allocation that has been running continuously the longest.
	Reconcile string `mapstructure:"reconcile" hcl:"reconcile,optional"`
}

func (ds *DisconnectStrategy) Validate(job *Job) error {
	var mErr *multierror.Error

	if ds.StopOnClientAfter != nil {
		if *ds.StopOnClientAfter < 0 {
			mErr = multierror.Append(mErr, errNegativeStopAfter)
		}

		if job.Type != JobTypeService && job.Type != JobTypeBatch {
			mErr = multierror.Append(mErr, errStopAfterNonService)
		}
	}

	if ds.LostAfter < 0 {
		mErr = multierror.Append(mErr, errNegativeLostAfter)
	}

	if ds.StopOnClientAfter != nil && ds.LostAfter != 0 {
		return multierror.Append(mErr, errStopAndLost)
	}

	switch ds.Reconcile {
		case "", ReconcileOptionBestScore, ReconcileOptionLongestRunning, ReconcileOptionKeepOriginal, ReconcileOptionKeepReplacement:
		default:
			return multierror.Append(mErr, fmt.Errorf("%w: %s", errInvalidReconcile, ds.Reconcile))
		}
	}

	return mErr.ErrorOrNil()
}

func (ds *DisconnectStrategy) Copy() *DisconnectStrategy {
	if ds == nil {
		return nil
	}

	nds := new(DisconnectStrategy)
	*nds = *ds
	return nds
}

func (ds *DisconnectStrategy) Canonicalize() {
	if ds.Replace == nil {
		ds.Replace = pointer.Of(true)
	}

	if ds.Reconcile == "" {
		ds.Reconcile = ReconcileOptionBestScore
	}
}
