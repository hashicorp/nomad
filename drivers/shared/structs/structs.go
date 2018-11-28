package structs

import "errors"

// CheckBufSize is the size of the buffer that is used for job output
const CheckBufSize = 4 * 1024

// DriverStatsNotImplemented is the error to be returned if a driver doesn't
// implement stats.
var DriverStatsNotImplemented = errors.New("stats not implemented for driver")
