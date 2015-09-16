package command

import (
	"testing"
)

func TestHelpers_FormatKV(t *testing.T) {
	in := []string{"alpha|beta", "charlie|delta"}
	out := formatKV(in)

	expect := "alpha   = beta\n"
	expect += "charlie = delta"

	if out != expect {
		t.Fatalf("expect: %s, got: %s", expect, out)
	}
}

func TestHelpers_FormatList(t *testing.T) {
	in := []string{"alpha|beta||delta"}
	out := formatList(in)

	expect := "alpha  beta  <none>  delta"

	if out != expect {
		t.Fatalf("expect: %s, got: %s", expect, out)
	}
}
