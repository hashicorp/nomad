// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

/*
*
csimanager manages locally running CSI Plugins on a Nomad host, and provides a
few different interfaces.

It provides:
  - a pluginmanager.PluginManager implementation that is used to fingerprint and
    heartbeat local node plugins
  - (TODO) a csimanager.AttachmentWaiter implementation that can be used to wait for an
    external CSIVolume to be attached to the node before returning
  - (TODO) a csimanager.NodeController implementation that is used to manage the node-local
    portions of the CSI specification, and encompassess volume staging/publishing
  - (TODO) a csimanager.VolumeChecker implementation that can be used by hooks to ensure
    their volumes are healthy(ish)
*/
package csimanager
