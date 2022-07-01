package structs

// MaxUUIDsPerWriteRequest is the maximum number of UUIDs that can be included
// within a single write request. This is to ensure that the Raft message does
// not become too large. The resulting value corresponds to 0.25MB of IDs or
// 7282 UUID strings.
var MaxUUIDsPerWriteRequest = (1024 * 256) / 36
