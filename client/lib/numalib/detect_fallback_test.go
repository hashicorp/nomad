package numalib

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_Fallback_yes(t *testing.T) {
	original := new(Topology)
	fallback := Fallback(original)
	must.NotEqOp(t, original, fallback) // pointer is replaced
	must.Len(t, 1, fallback.Cores)
}

func Test_Fallback_no(t *testing.T) {
	original := Scan(PlatformScanners())
	fallback := Fallback(original)
	must.EqOp(t, original, fallback) // pointer is same
}
