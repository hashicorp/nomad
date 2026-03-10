// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package drivers

import "errors"

var ErrTaskNotFound = errors.New("task not found for given id")

var ErrChannelClosed = errors.New("channel closed")

var DriverRequiresRootMessage = "Driver must run as root"

var NoCgroupMountMessage = "Failed to discover cgroup mount point"

var CgroupMountEmpty = "Cgroup mount point unavailable"
