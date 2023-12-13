// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

// MaxUUIDsPerWriteRequest is the maximum number of UUIDs that can be included
// within a single write request. This is to ensure that the Raft message does
// not become too large. The resulting value corresponds to 0.25MB of IDs or
// 7281 UUID strings.
const MaxUUIDsPerWriteRequest = 7281 // (1024 * 256) / 36
