// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package group

import "sync"

// group wraps a func() in a goroutine and provides a way to block until it
// exits. Inspired by https://godoc.org/golang.org/x/sync/errgroup
type Group struct {
	wg sync.WaitGroup
}

// Go starts f in a goroutine and must be called before Wait.
func (g *Group) Go(f func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		f()
	}()
}

func (g *Group) AddCh(ch <-chan struct{}) {
	g.Go(func() {
		<-ch
	})
}

// Wait for all goroutines to exit. Must be called after all calls to Go
// complete.
func (g *Group) Wait() {
	g.wg.Wait()
}
