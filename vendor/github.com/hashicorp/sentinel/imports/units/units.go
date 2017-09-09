package units

import (
	"time"
)

// Byte sizes (base 2 form)
const (
	B  int64 = iota
	KB int64 = 1 << (10 * iota)
	MB
	GB
	TB
	PB
)

// unitMap is the lookup table for the units
var unitMap = map[string]interface{}{
	// Byte sizes
	"byte":     B,
	"kilobyte": KB,
	"megabyte": MB,
	"gigabyte": GB,
	"terabyte": TB,
	"petabyte": PB,

	// Time
	"nanosecond":  time.Nanosecond,
	"microsecond": time.Microsecond,
	"millisecond": time.Millisecond,
	"second":      time.Second,
	"minute":      time.Minute,
	"hour":        time.Hour,
}
