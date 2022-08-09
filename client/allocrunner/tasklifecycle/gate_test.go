package tasklifecycle

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
)

func TestGate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name string
		test func(*testing.T, *Gate)
	}{
		{
			name: "starts blocked",
			test: func(t *testing.T, g *Gate) {
				requireChannelBlocking(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "block",
			test: func(t *testing.T, g *Gate) {
				g.Close()
				requireChannelBlocking(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "allow",
			test: func(t *testing.T, g *Gate) {
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "block twice",
			test: func(t *testing.T, g *Gate) {
				g.Close()
				g.Close()
				requireChannelBlocking(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "allow twice",
			test: func(t *testing.T, g *Gate) {
				g.Open()
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "allow block allow",
			test: func(t *testing.T, g *Gate) {
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "first allow")
				g.Close()
				requireChannelBlocking(t, g.WaitCh(), "block")
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "second allow")
			},
		},
		{
			name: "concurrent access",
			test: func(t *testing.T, g *Gate) {
				x := 100
				go func() {
					for i := 0; i < x; i++ {
						g.Open()
					}
				}()
				go func() {
					for i := 0; i < x/10; i++ {
						g.Close()
					}
				}()
				requireChannelPassing(t, g.WaitCh(), "gate should be open")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shutdownCh := make(chan struct{})
			defer close(shutdownCh)

			g := NewGate(shutdownCh)
			tc.test(t, g)
		})
	}
}
