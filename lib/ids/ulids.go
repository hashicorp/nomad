package ids

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/hashicorp/go-uuid"
	"oss.indeed.com/go/libtime"
)

// NewULID creates a new pseudo-ULID based on the ulid/spec. The output format
// is that of a UUID rather than the compact form of a ULID.
//
// Specification of ULID:
// https://github.com/ulid/spec
//
// This implementation *does not* guarantee monotonic increasing IDs within the
// same millisecond.
//
// This implementation *is* safe to use across goroutines.
func NewULID() string {
	b := make([]byte, 16)

	// first 6 bytes are based on timestamp
	ms := libtime.ToMilliseconds(time.Now().UTC())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// last 10 bytes are based on true random
	n, rndErr := rand.Read(b[6:])
	if rndErr != nil {
		panic(fmt.Errorf("failed to generate ulid: %v", rndErr))
	}
	if n != 10 {
		panic("failed to generate ulid: not enough random bytes")
	}

	// we like our ULIDs formatted as UUIDs
	s, fmtErr := uuid.FormatUUID(b)
	if fmtErr != nil {
		panic(fmt.Errorf("failed to format ulid as uuid: %v", fmtErr))
	}

	return s
}
