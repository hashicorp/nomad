// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mounter

import (
	"context"
	"fmt"
	"net/rpc"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/helper/users"
)

type Alloc struct {
	ID       string // alloc ID
	AllocDir *allocdir.AllocDir
	TaskDirs map[string]*allocdir.TaskDir
}

type MounterServer struct {
	log     hclog.Logger
	udsPath string
	user    string

	allocLock sync.RWMutex
	allocs    map[string]*Alloc

	rpcServer *rpc.Server
}

func NewMounterServer(logger hclog.Logger, udsPath, user string) (*MounterServer, error) {

	if udsPath == "" {
		udsPath = defaultUDSPath
	}

	ms := &MounterServer{
		log:     logger,
		udsPath: udsPath,
		user:    user,
		allocs:  map[string]*Alloc{},
	}

	rpcSrv := rpc.NewServer()
	err := rpcSrv.Register(&MounterEndpoint{
		srv:   ms,
		inner: &allocdir.DefaultBuilder{},
	})
	if err != nil {
		return nil, err
	}
	ms.rpcServer = rpcSrv
	return ms, nil
}

func (ms *MounterServer) Run(ctx context.Context) error {

	// TODO: SocketFileFor fails wide-open if we're not running as root
	listener, err := users.SocketFileFor(ms.log, ms.udsPath, ms.user)
	if err != nil {
		return fmt.Errorf("error creating mounter socket: %s: %w", ms.udsPath, err)
	}

	ms.log.Info("listening for mounts")

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			ms.log.Trace("error accepting request on mounter API", "error", err)
			return nil
		}
		go ms.rpcServer.ServeConn(conn)
	}
}
