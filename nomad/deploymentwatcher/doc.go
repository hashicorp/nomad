// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// deploymentwatcher creates and tracks Deployments, which hold meta data describing the
// process of upgrading a running job to a new set of Allocations. This encompasses settings
// for canary deployments and blue/green rollouts.
//
// - The watcher is only enabled on the active raft leader.
// - func (w *deploymentWatcher) watch() is the main deploymentWatcher process
package deploymentwatcher
