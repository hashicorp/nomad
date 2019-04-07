package drivers

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers/proto"
)

// serverExecTaskHelper is a convenience helper to ensure that
// server sends and requests all happen in dedicated goroutines
type serverExecTaskHelper struct {
	server proto.Driver_ExecTaskStreamingServer

	setup       *proto.ExecTaskStreamingRequest
	setupRecvCh chan interface{}

	sendCh chan *proto.ExecTaskStreamingResponse
	recvCh chan *proto.ExecTaskStreamingRequest

	sendDoneCh chan interface{}
	recvDoneCh chan error
}

func (h *serverExecTaskHelper) start() {
	go h.sendGoroutine()
	go h.recvGoroutine()
}

func (h *serverExecTaskHelper) sendGoroutine() {
	defer close(h.sendDoneCh)

	for m := range h.sendCh {
		h.server.Send(m)
	}
}

func (h *serverExecTaskHelper) recvGoroutine() {
	defer close(h.recvDoneCh)

	msg, err := h.server.Recv()
	if err != nil {
		h.recvDoneCh <- fmt.Errorf("failed to receive initial message: %v")
		close(h.setupRecvCh)
		return
	}

	if msg.Setup == nil {
		h.recvDoneCh <- fmt.Errorf("first message should always be setup")
		close(h.setupRecvCh)
		return
	}

	h.setup = msg
	close(h.setupRecvCh)

	for {
		msg, err := h.server.Recv()
		if err != nil {
			h.recvDoneCh <- fmt.Errorf("failed to receive message: %v", err)
			return
		}

		h.recvCh <- msg
	}
}

func (h *serverExecTaskHelper) setupMessage() (*proto.ExecTaskStreamingRequest, error) {
	// wait until setup is set
	<-h.setupRecvCh

	if h.setup != nil {
		return h.setup, nil
	}

	select {
	case err := <-h.recvDoneCh:
		if err != nil {
			return nil, err
		}
	default:
	}

	return nil, fmt.Errorf("unexpeced error getting setup")
}

func (h *serverExecTaskHelper) drainSendQueue(timeout time.Duration) {
	select {
	case <-h.sendDoneCh:
	case <-time.After(timeout):
	}
}

func newExecTaskHelper(server proto.Driver_ExecTaskStreamingServer) *serverExecTaskHelper {
	r := &serverExecTaskHelper{
		server:      server,
		setupRecvCh: make(chan interface{}),
		sendCh:      make(chan *proto.ExecTaskStreamingResponse, 10),
		recvCh:      make(chan *proto.ExecTaskStreamingRequest, 10),
		sendDoneCh:  make(chan interface{}),
		recvDoneCh:  make(chan error, 1),
	}
	r.start()
	return r
}
