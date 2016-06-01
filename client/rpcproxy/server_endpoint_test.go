package rpcproxy_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/hashicorp/nomad/client/rpcproxy"
)

// func (k *rpcproxy.EndpointKey) Equal(x *rpcproxy.EndpointKey) {
func TestServerEndpointKey_Equal(t *testing.T) {
	tests := []struct {
		name  string
		s1    *rpcproxy.ServerEndpoint
		s2    *rpcproxy.ServerEndpoint
		equal bool
	}{
		{
			name:  "equal",
			s1:    &rpcproxy.ServerEndpoint{Name: "k1"},
			s2:    &rpcproxy.ServerEndpoint{Name: "k1"},
			equal: true,
		},
		{
			name:  "not equal",
			s1:    &rpcproxy.ServerEndpoint{Name: "k1"},
			s2:    &rpcproxy.ServerEndpoint{Name: "k2"},
			equal: false,
		},
	}

	for _, test := range tests {
		if test.s1.Key().Equal(test.s2.Key()) != test.equal {
			t.Errorf("fixture %s failed forward comparison", test.name)
		}

		if test.s2.Key().Equal(test.s1.Key()) != test.equal {
			t.Errorf("fixture %s failed reverse comparison", test.name)
		}
	}
}

// func (k *rpcproxy.ServerEndpoint) String() {
func TestServerEndpoint_String(t *testing.T) {
	tests := []struct {
		name string
		s    *rpcproxy.ServerEndpoint
		str  string
	}{
		{
			name: "name",
			s:    &rpcproxy.ServerEndpoint{Name: "s"},
			str:  "s (:)",
		},
		{
			name: "name, host, port",
			s: &rpcproxy.ServerEndpoint{
				Name: "s",
				Host: "127.0.0.1",
				Port: "4647",
			},
			str: "s (tcp:127.0.0.1:4647)",
		},
	}

	for _, test := range tests {
		if test.s.Addr == nil && (test.s.Host != "" && test.s.Port != "") {
			fmt.Printf("Setting addr\n")
			addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(test.s.Host, test.s.Port))
			if err == nil {
				test.s.Addr = addr
			}
		}
		if test.s.String() != test.str {
			t.Errorf("fixture %q failed: %q vs %q", test.name, test.s.String(), test.str)
		}
	}
}
