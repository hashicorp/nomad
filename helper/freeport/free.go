package freeport

import (
	"fmt"
	"net"

	"github.com/mitchellh/go-testing-interface"
)

func Port() (int, error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, err
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return -1, fmt.Errorf("unexpected address type: %T", ln.Addr())
	}

	return addr.Port, nil
}

func Get(t testing.T) int {
	t.Helper()

	p, err := Port()
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}

	return p
}
