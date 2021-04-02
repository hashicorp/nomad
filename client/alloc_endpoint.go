package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-msgpack/codec"

	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// Allocations endpoint is used for interacting with client allocations
type Allocations struct {
	c *Client
}

func NewAllocationsEndpoint(c *Client) *Allocations {
	a := &Allocations{c: c}
	a.c.streamingRpcs.Register("Allocations.Exec", a.exec)
	return a
}

// GarbageCollectAll is used to garbage collect all allocations on a client.
func (a *Allocations) GarbageCollectAll(args *nstructs.NodeSpecificRequest, reply *nstructs.GenericResponse) error {
	defer metrics.MeasureSince([]string{"client", "allocations", "garbage_collect_all"}, time.Now())

	// Check node write permissions
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return nstructs.ErrPermissionDenied
	}

	a.c.CollectAllAllocs()
	return nil
}

// GarbageCollect is used to garbage collect an allocation on a client.
func (a *Allocations) GarbageCollect(args *nstructs.AllocSpecificRequest, reply *nstructs.GenericResponse) error {
	defer metrics.MeasureSince([]string{"client", "allocations", "garbage_collect"}, time.Now())

	alloc, err := a.c.GetAlloc(args.AllocID)
	if err != nil {
		return err
	}

	// Check namespace submit job permission.
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return nstructs.ErrPermissionDenied
	}

	if !a.c.CollectAllocation(args.AllocID) {
		return fmt.Errorf("No such allocation on client, or allocation not eligible for GC")
	}

	return nil
}

// Signal is used to send a signal to an allocation's tasks on a client.
func (a *Allocations) Signal(args *nstructs.AllocSignalRequest, reply *nstructs.GenericResponse) error {
	defer metrics.MeasureSince([]string{"client", "allocations", "signal"}, time.Now())

	alloc, err := a.c.GetAlloc(args.AllocID)
	if err != nil {
		return err
	}

	// Check namespace alloc-lifecycle permission.
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityAllocLifecycle) {
		return nstructs.ErrPermissionDenied
	}

	return a.c.SignalAllocation(args.AllocID, args.Task, args.Signal)
}

// Restart is used to trigger a restart of an allocation or a subtask on a client.
func (a *Allocations) Restart(args *nstructs.AllocRestartRequest, reply *nstructs.GenericResponse) error {
	defer metrics.MeasureSince([]string{"client", "allocations", "restart"}, time.Now())

	alloc, err := a.c.GetAlloc(args.AllocID)
	if err != nil {
		return err
	}

	// Check namespace alloc-lifecycle permission.
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityAllocLifecycle) {
		return nstructs.ErrPermissionDenied
	}

	return a.c.RestartAllocation(args.AllocID, args.TaskName)
}

// Stats is used to collect allocation statistics
func (a *Allocations) Stats(args *cstructs.AllocStatsRequest, reply *cstructs.AllocStatsResponse) error {
	defer metrics.MeasureSince([]string{"client", "allocations", "stats"}, time.Now())

	alloc, err := a.c.GetAlloc(args.AllocID)
	if err != nil {
		return err
	}

	// Check read-job permission.
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityReadJob) {
		return nstructs.ErrPermissionDenied
	}

	clientStats := a.c.StatsReporter()
	aStats, err := clientStats.GetAllocStats(args.AllocID)
	if err != nil {
		return err
	}

	stats, err := aStats.LatestAllocStats(args.Task)
	if err != nil {
		return err
	}

	reply.Stats = stats
	return nil
}

// exec is used to execute command in a running task
func (a *Allocations) exec(conn io.ReadWriteCloser) {
	defer metrics.MeasureSince([]string{"client", "allocations", "exec"}, time.Now())
	defer conn.Close()

	execID := uuid.Generate()
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	code, err := a.execImpl(encoder, decoder, execID)
	if err != nil {
		a.c.logger.Info("task exec session ended with an error", "error", err, "code", code)
		handleStreamResultError(err, code, encoder)
		return
	}

	a.c.logger.Info("task exec session ended", "exec_id", execID)
}

func (a *Allocations) execImpl(encoder *codec.Encoder, decoder *codec.Decoder, execID string) (code *int64, err error) {

	// Decode the arguments
	var req cstructs.AllocExecRequest
	if err := decoder.Decode(&req); err != nil {
		return helper.Int64ToPtr(500), err
	}

	if a.c.GetConfig().DisableRemoteExec {
		return nil, structs.ErrPermissionDenied
	}

	if req.AllocID == "" {
		return helper.Int64ToPtr(400), allocIDNotPresentErr
	}
	ar, err := a.c.getAllocRunner(req.AllocID)
	if err != nil {
		code := helper.Int64ToPtr(500)
		if structs.IsErrUnknownAllocation(err) {
			code = helper.Int64ToPtr(404)
		}

		return code, err
	}
	alloc := ar.Alloc()

	aclObj, token, err := a.c.resolveTokenAndACL(req.QueryOptions.AuthToken)
	{
		// log access
		tokenName, tokenID := "", ""
		if token != nil {
			tokenName, tokenID = token.Name, token.AccessorID
		}

		a.c.logger.Info("task exec session starting",
			"exec_id", execID,
			"alloc_id", req.AllocID,
			"task", req.Task,
			"command", req.Cmd,
			"tty", req.Tty,
			"access_token_name", tokenName,
			"access_token_id", tokenID,
		)
	}

	// Check alloc-exec permission.
	if err != nil {
		return nil, err
	} else if aclObj != nil && !aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityAllocExec) {
		return nil, structs.ErrPermissionDenied
	}

	// Validate the arguments
	if req.Task == "" {
		return helper.Int64ToPtr(400), taskNotPresentErr
	}
	if len(req.Cmd) == 0 {
		return helper.Int64ToPtr(400), errors.New("command is not present")
	}

	capabilities, err := ar.GetTaskDriverCapabilities(req.Task)
	if err != nil {
		code := helper.Int64ToPtr(500)
		if structs.IsErrUnknownAllocation(err) {
			code = helper.Int64ToPtr(404)
		}

		return code, err
	}

	// check node access
	if aclObj != nil && capabilities.FSIsolation == drivers.FSIsolationNone {
		exec := aclObj.AllowNsOp(alloc.Namespace, acl.NamespaceCapabilityAllocNodeExec)
		if !exec {
			return nil, structs.ErrPermissionDenied
		}
	}

	allocState, err := a.c.GetAllocState(req.AllocID)
	if err != nil {
		code := helper.Int64ToPtr(500)
		if structs.IsErrUnknownAllocation(err) {
			code = helper.Int64ToPtr(404)
		}

		return code, err
	}

	// Check that the task is there
	taskState := allocState.TaskStates[req.Task]
	if taskState == nil {
		return helper.Int64ToPtr(400), fmt.Errorf("unknown task name %q", req.Task)
	}

	if taskState.StartedAt.IsZero() {
		return helper.Int64ToPtr(404), fmt.Errorf("task %q not started yet.", req.Task)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := ar.GetTaskExecHandler(req.Task)
	if h == nil {
		return helper.Int64ToPtr(404), fmt.Errorf("task %q is not running.", req.Task)
	}

	err = h(ctx, req.Cmd, req.Tty, newExecStream(decoder, encoder))
	if err != nil {
		code := helper.Int64ToPtr(500)
		return code, err
	}

	return nil, nil
}

// newExecStream returns a new exec stream as expected by drivers that interpolate with RPC streaming format
func newExecStream(decoder *codec.Decoder, encoder *codec.Encoder) drivers.ExecTaskStream {
	buf := new(bytes.Buffer)
	return &execStream{
		decoder: decoder,

		buf:        buf,
		encoder:    encoder,
		frameCodec: codec.NewEncoder(buf, nstructs.JsonHandle),
	}
}

type execStream struct {
	decoder *codec.Decoder

	encoder    *codec.Encoder
	buf        *bytes.Buffer
	frameCodec *codec.Encoder
}

// Send sends driver output response across RPC mechanism using cstructs.StreamErrWrapper
func (s *execStream) Send(m *drivers.ExecTaskStreamingResponseMsg) error {
	s.buf.Reset()
	s.frameCodec.Reset(s.buf)

	s.frameCodec.MustEncode(m)
	return s.encoder.Encode(cstructs.StreamErrWrapper{
		Payload: s.buf.Bytes(),
	})
}

// Recv returns next exec user input from the RPC to be passed to driver exec handler
func (s *execStream) Recv() (*drivers.ExecTaskStreamingRequestMsg, error) {
	req := drivers.ExecTaskStreamingRequestMsg{}
	err := s.decoder.Decode(&req)
	return &req, err
}
