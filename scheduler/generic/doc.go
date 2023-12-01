// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package generic provides functionality that is specific to the service and
// batch schedulers. These schedulers are wrapped via the generic scheduler,
// which is why this package is named as such. It should not be shared or used
// by the system or sysbatch schedulers.
package generic
