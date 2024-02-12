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

func NewDisconnectStrategy() *DisconnectStrategy {
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
	StopAfterOnClient *time.Duration `mapstructure:"stop_after" hcl:"stop_after_client"`

	// A boolean field used to define if the allocations should be replaced while
	// its  considered disconnected.
	Replace *bool `mapstructure:"replace" hcl:"replace,optional"`

	// Once the disconnected node starts reporting again, it will define which
	// instances to keep: the original allocations, the replacement, the one
	// running on the node with the best score as it is currently implemented,
	// or the allocation that has been running continuously the longest.
	Reconcile string `mapstructure:"reconcile" hcl:"reconcile,optional"`
}

func (ds *DisconnectStrategy) Validate(job *Job) error {
	var mErr *multierror.Error

	if ds.StopAfterOnClient != nil {
		if *ds.StopAfterOnClient < 0 {
			mErr = multierror.Append(mErr, errNegativeStopAfter)
		}

		if job.Type != JobTypeService && job.Type != JobTypeBatch {
			mErr = multierror.Append(mErr, errStopAfterNonService)
		}
	}

	if ds.LostAfter < 0 {
		mErr = multierror.Append(mErr, errNegativeLostAfter)
	}

	if ds.StopAfterOnClient != nil && ds.LostAfter != 0 {
		return multierror.Append(mErr, errStopAndLost)
	}

	if ds.Reconcile != "" &&
		ds.Reconcile != ReconcileOptionBestScore &&
		ds.Reconcile != ReconcileOptionLongestRunning &&
		ds.Reconcile != ReconcileOptionKeepOriginal &&
		ds.Reconcile != ReconcileOptionKeepReplacement {
		return multierror.Append(mErr, fmt.Errorf("%w: %s", errInvalidReconcile, ds.Reconcile))
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
	cds := NewDisconnectStrategy()
	if ds.LostAfter == 0 {
		ds.LostAfter = cds.LostAfter
	}

	if ds.StopAfterOnClient != nil {
		ds.StopAfterOnClient = cds.StopAfterOnClient
	}

	if ds.Replace == nil {
		ds.Replace = cds.Replace
	}

	if ds.Reconcile != ReconcileOptionBestScore {
		ds.Reconcile = cds.Reconcile
	}
}
