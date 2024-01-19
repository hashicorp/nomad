// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package anonymous

const (
	// SystemMax is that maximum allowable UID/GID for anonymous users.
	//
	// https://systemd.io/UIDS-GIDS/
	SystemMax = 4294967295 // (2^32)-1

	// SystemMin is the minimum allowable UID/GID for anonymous users.
	//
	// https://systemd.io/UIDS-GIDS/
	SystemMin = 65536 // first unused by systemd
)
