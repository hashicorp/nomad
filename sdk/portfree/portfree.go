// Package portfree provides a helper for reserving free TCP ports across multiple
// processes on the same machine. This re-implementation of freeport works by asking
// the kernel for available ports in the ephemeral port range. It does so by binding
// to an address with port 0 (e.g. 127.0.0.1:0), modifying the socket to disable SO_LINGER,
// close the connection, and finally return the port that was used. This *probably*
// works well, because the kernel re-uses ports in an LRU fashion, implying the
// test code asking for the ports *should* be the only thing immediately asking
// to bind that port again.
//
// Package portfree is meant to be useful when the code being tested does not accept
// a net.Listener. Any code that accepts a net.Listener (or uses net/http/httptest.Server)
// can use port 0 (ex: 127.0.0.1:0) to find an unused ephemeral port that will
// not conflict, without the risky shenanigans needed by freeport.
package portfree

import (
	"net"
	"strconv"
	"testing"
)

const (
	defaultAddress = "127.0.0.1"
)

type Getter interface {
	Get(n int) []int
	GetOne() int
}

func New(t *testing.T, opts ...Option) Getter {
	g := &getter{
		t:  t,
		ip: net.ParseIP(defaultAddress),
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

type getter struct {
	t  *testing.T
	ip net.IP
}

type Option func(Getter)

func WithAddress(address string) Option {
	return func(g Getter) {
		g.(*getter).ip = net.ParseIP(address)
	}
}

func (g *getter) Get(n int) []int {
	ports := make([]int, 0, n)
	for i := 0; i < n; i++ {
		ports = append(ports, g.GetOne())
	}
	return ports
}

func (g *getter) GetOne() int {
	tcpAddr := &net.TCPAddr{IP: g.ip, Port: 0}
	l, listenErr := net.ListenTCP("tcp", tcpAddr)
	if listenErr != nil {
		g.t.Fatalf("failed to aqcuire port: %v", listenErr)
	}

	if setErr := setSocketOpt(l); setErr != nil {
		g.t.Fatalf("failed to modify socket: %v", setErr)
	}

	_, port, splitErr := net.SplitHostPort(l.Addr().String())
	if splitErr != nil {
		g.t.Fatalf("failed to parse acuired address: %v", splitErr)
	}

	p, parseErr := strconv.Atoi(port)
	if parseErr != nil {
		g.t.Fatalf("failed to parse acquired port: %v", parseErr)
	}

	closeErr := l.Close()
	if closeErr != nil {
		g.t.Fatalf("failed to close acuired port: %v", closeErr)
	}

	return p
}
