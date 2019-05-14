package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
)

func TestNetworkFingerPrint_linkspeed_parse(t *testing.T) {
	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &DefaultNetworkInterfaceDetector{}}

	var outputTests = []struct {
		in  string
		out int
	}{
		{"10 Mbps", 10},
		{"2 bps", 0},
		{"1 Gbps", 1000},
		{"2Mbps", 0},
		{"1000 Kbps", 1},
		{"1 Kbps", 0},
		{"0 Mbps", 0},
		{"2 2 Mbps", 0},
		{"a Mbps", 0},
		{"1 Tbps", 0},
	}

	for _, ot := range outputTests {
		out := f.parseLinkSpeed(ot.in)
		if out != ot.out {
			t.Errorf("parseLinkSpeed(%s) => %d, should be %d", ot.in, out, ot.out)
		}
	}
}
