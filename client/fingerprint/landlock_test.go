package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/go-landlock"
	"github.com/shoenig/test/must"
)

func TestLandlockFingerprint(t *testing.T) {
	ci.Parallel(t)

	version, err := landlock.Detect()
	must.NoError(t, err)

	logger := testlog.HCLogger(t)
	f := NewLandlockFingerprint(logger)

	var response FingerprintResponse
	f.Fingerprint(nil, &response)

	result := response.Attributes[landlockKey]
	exp := map[int]string{
		0: "unavailable",
		1: "v1",
		2: "v2",
		3: "v3",
	}
	must.Eq(t, exp[version], result)
}
