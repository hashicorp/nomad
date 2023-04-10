// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package drivers

import "fmt"

var ErrTaskNotFound = fmt.Errorf("task not found for given id")

var DriverRequiresRootMessage = "Driver must run as root"

var NoCgroupMountMessage = "Failed to discover cgroup mount point"

var CgroupMountEmpty = "Cgroup mount point unavailable"
