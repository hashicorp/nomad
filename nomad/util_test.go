package nomad

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/serf/serf"
)

func TestIsNomadServer(t *testing.T) {
	m := serf.Member{
		Name: "foo",
		Addr: net.IP([]byte{127, 0, 0, 1}),
		Tags: map[string]string{
			"role":   "nomad",
			"region": "aws",
			"dc":     "east-aws",
			"port":   "10000",
			"vsn":    "1",
		},
	}
	valid, parts := isNomadServer(m)
	if !valid || parts.Region != "aws" ||
		parts.Datacenter != "east-aws" || parts.Port != 10000 {
		t.Fatalf("bad: %v %v", valid, parts)
	}
	if parts.Name != "foo" {
		t.Fatalf("bad: %v", parts)
	}
	if parts.Bootstrap {
		t.Fatalf("unexpected bootstrap")
	}
	if parts.Expect != 0 {
		t.Fatalf("bad: %v", parts.Expect)
	}

	m.Tags["bootstrap"] = "1"
	valid, parts = isNomadServer(m)
	if !valid || !parts.Bootstrap {
		t.Fatalf("expected bootstrap")
	}
	if parts.Addr.String() != "127.0.0.1:10000" {
		t.Fatalf("bad addr: %v", parts.Addr)
	}
	if parts.Version != 1 {
		t.Fatalf("bad: %v", parts)
	}

	m.Tags["expect"] = "3"
	delete(m.Tags, "bootstrap")
	valid, parts = isNomadServer(m)
	if !valid || parts.Expect != 3 {
		t.Fatalf("bad: %v", parts.Expect)
	}
}

func TestRandomStagger(t *testing.T) {
	intv := time.Minute
	for i := 0; i < 10; i++ {
		stagger := randomStagger(intv)
		if stagger < 0 || stagger >= intv {
			t.Fatalf("Bad: %v", stagger)
		}
	}
}

func TestShuffleStrings(t *testing.T) {
	// Generate input
	inp := make([]string, 10)
	for idx := range inp {
		inp[idx] = structs.GenerateUUID()
	}

	// Copy the input
	orig := make([]string, len(inp))
	copy(orig, inp)

	// Shuffle
	shuffleStrings(inp)

	// Ensure order is not the same
	if reflect.DeepEqual(inp, orig) {
		t.Fatalf("shuffle failed")
	}
}

func TestMaxUint64(t *testing.T) {
	if maxUint64(1, 2) != 2 {
		t.Fatalf("bad")
	}
	if maxUint64(2, 2) != 2 {
		t.Fatalf("bad")
	}
	if maxUint64(2, 1) != 2 {
		t.Fatalf("bad")
	}
}

func TestRateScaledInterval(t *testing.T) {
	min := 1 * time.Second
	rate := 200.0
	if v := rateScaledInterval(rate, min, 0); v != min {
		t.Fatalf("Bad: %v", v)
	}
	if v := rateScaledInterval(rate, min, 100); v != min {
		t.Fatalf("Bad: %v", v)
	}
	if v := rateScaledInterval(rate, min, 200); v != 1*time.Second {
		t.Fatalf("Bad: %v", v)
	}
	if v := rateScaledInterval(rate, min, 1000); v != 5*time.Second {
		t.Fatalf("Bad: %v", v)
	}
	if v := rateScaledInterval(rate, min, 5000); v != 25*time.Second {
		t.Fatalf("Bad: %v", v)
	}
	if v := rateScaledInterval(rate, min, 10000); v != 50*time.Second {
		t.Fatalf("Bad: %v", v)
	}
}
