// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mounter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
)

type MounterClient struct {
	log     hclog.Logger
	udsPath string
	lock    sync.RWMutex
	inner   *rpc.Client
}

var clientConnectTimeout = time.Second * 5

func NewMounterClient(shutdownCh chan struct{}, log hclog.Logger, udsPath string) (*MounterClient, error) {

	// we don't have ctx threaded down from the client, so we need to mock one
	// up here
	ctx, cancel := context.WithCancel(context.TODO())
	go func() {
		select {
		case <-shutdownCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// TODO: this appears to hang longer than timeout if the mounter isn't
	// up. need to fix that
	inner, err := connect(ctx, udsPath)
	if err != nil {
		cancel()
	}

	c := &MounterClient{inner: inner, udsPath: udsPath, log: log}

	var reply MounterResp
	err = RPC(c, MethodPing, &MounterPingReq{}, &reply)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func connect(ctx context.Context, udsPath string) (*rpc.Client, error) {
	var inner *rpc.Client
	var err error
	err = helper.WithBackoffFunc(ctx, time.Millisecond*100, clientConnectTimeout, func() error {
		inner, err = rpc.Dial("unix", udsPath)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return inner, nil
}

func RPC[A, B any](c *MounterClient, method string, args A, reply B) error {
	c.log.Info("sending RPC to mounter", "method", method)
	c.lock.RLock()
	err := c.inner.Call(method, args, reply)
	c.lock.RUnlock()
	if err != nil {
		if errors.Is(err, net.ErrClosed) || errors.Is(err, rpc.ErrShutdown) {
			// if the mounter restarts, we want to try to reconnect to it
			ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
			defer cancel()
			inner, err := connect(ctx, c.udsPath)
			if err != nil {
				return fmt.Errorf(
					"connection to mounter was lost and could not be restored %w", err)
			}

			c.lock.Lock()
			c.inner = inner
			c.lock.Unlock()
			c.log.Debug("connection to mounter was restored")
			err = c.inner.Call(method, args, reply)
			if err != nil {
				return fmt.Errorf("mounter returned error: %w", err)
			}
		} else {
			return fmt.Errorf("could not send RPC to mounter: %w", err)
		}
	}

	c.log.Debug("client received reply", "reply", spew.Sdump(reply))
	return nil
}
