package portfree

import (
	"testing"
)

func TestGetter_New(t *testing.T) {
	g := New(t)

	ip := g.(*getter).ip.String()
	if ip != defaultAddress {
		t.Fatalf("expected default address to be %s, got: %s", defaultAddress, ip)
	}
}

func checkPort(t *testing.T, port int) {
	if !(port >= 1024) {
		t.Fatalf("expected port above 1024, got: %v", port)
	}
}

func TestGetter_GetOne(t *testing.T) {
	g := New(t)
	port := g.GetOne()
	checkPort(t, port)
}

func TestGetter_Get(t *testing.T) {
	g := New(t)
	ports := g.Get(5)
	for _, port := range ports {
		checkPort(t, port)
	}
}
