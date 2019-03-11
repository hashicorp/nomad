package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/acl"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/ugorji/go/codec"
)

// Allocations endpoint is used for interacting with client allocations
type Allocations struct {
	c *Client
}

func NewAllocationsEndpoint(c *Client) *Allocations {
	a := &Allocations{c: c}
	a.c.streamingRpcs.Register("Allocations.Exec", a.Exec)
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

	// Check submit job permissions
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return nstructs.ErrPermissionDenied
	}

	if !a.c.CollectAllocation(args.AllocID) {
		// Could not find alloc
		return nstructs.NewErrUnknownAllocation(args.AllocID)
	}

	return nil
}

// Restart is used to trigger a restart of an allocation or a subtask on a client.
func (a *Allocations) Restart(args *nstructs.AllocRestartRequest, reply *nstructs.GenericResponse) error {
	defer metrics.MeasureSince([]string{"client", "allocations", "restart"}, time.Now())

	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.Namespace, acl.NamespaceCapabilityAllocLifecycle) {
		return nstructs.ErrPermissionDenied
	}

	return a.c.RestartAllocation(args.AllocID, args.TaskName)
}

// Stats is used to collect allocation statistics
func (a *Allocations) Stats(args *cstructs.AllocStatsRequest, reply *cstructs.AllocStatsResponse) error {
	defer metrics.MeasureSince([]string{"client", "allocations", "stats"}, time.Now())

	// Check read job permissions
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.Namespace, acl.NamespaceCapabilityReadJob) {
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

func (a *Allocations) Exec(conn io.ReadWriteCloser) {
	defer metrics.MeasureSince([]string{"client", "allocations", "exec"}, time.Now())
	defer conn.Close()

	// Decode the arguments
	var req cstructs.AllocExecRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&req); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	a.c.logger.Info("received exec request", "req", fmt.Sprintf("%#v", req))

	// Check read permissions
	if aclObj, err := a.c.ResolveToken(req.QueryOptions.AuthToken); err != nil {
		handleStreamResultError(err, nil, encoder)
		return
	} else if aclObj != nil {
		exec := aclObj.AllowNsOp(req.QueryOptions.Namespace, acl.NamespaceCapabilityAllocExec)
		if !exec {
			handleStreamResultError(structs.ErrPermissionDenied, nil, encoder)
			return
		}
	}

	// Validate the arguments
	if req.AllocID == "" {
		handleStreamResultError(allocIDNotPresentErr, helper.Int64ToPtr(400), encoder)
		return
	}
	if req.Task == "" {
		handleStreamResultError(taskNotPresentErr, helper.Int64ToPtr(400), encoder)
		return
	}
	if len(req.Cmd) == 0 {
		handleStreamResultError(errors.New("command is not present"), helper.Int64ToPtr(400), encoder)
	}

	ar, err := a.c.getAllocRunner(req.AllocID)
	if err != nil {
		code := helper.Int64ToPtr(500)
		if structs.IsErrUnknownAllocation(err) {
			code = helper.Int64ToPtr(404)
		}

		handleStreamResultError(err, code, encoder)
		return
	}

	allocState, err := a.c.GetAllocState(req.AllocID)
	if err != nil {
		code := helper.Int64ToPtr(500)
		if structs.IsErrUnknownAllocation(err) {
			code = helper.Int64ToPtr(404)
		}

		handleStreamResultError(err, code, encoder)
		return
	}

	// Check that the task is there
	taskState := allocState.TaskStates[req.Task]
	if taskState == nil {
		handleStreamResultError(
			fmt.Errorf("unknown task name %q", req.Task),
			helper.Int64ToPtr(400),
			encoder)
		return
	}

	if taskState.StartedAt.IsZero() {
		handleStreamResultError(
			fmt.Errorf("task %q not started yet.", req.Task),
			helper.Int64ToPtr(404),
			encoder)
		return
	}

	resizeCh := make(chan drivers.TerminalSize, 1)

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()

	ctx, ctxCancel := context.WithCancel(context.Background())

	cancel := func() {
		ctxCancel()
		outReader.Close()
		errReader.Close()
		inWriter.Close()
		close(resizeCh)
	}

	// Create a goroutine to detect the remote side closing

	// process input
	go func() {
		frame := &sframer.StreamFrame{}
		for {
			frame.Clear()
			err := decoder.Decode(frame)
			if err == io.EOF {
				a.c.logger.Warn("connection closed")
				cancel()
				break
			}
			if err != nil {
				a.c.logger.Warn("received unexpected error", "error", err)
				break
			}
			a.c.logger.Warn("received input", "input", fmt.Sprintf("%#v", frame), "error", err)
			switch {
			case frame.File == "stdin":
				inWriter.Write(frame.Data)
			case frame.FileEvent == "resize":
				t := drivers.TerminalSize{}
				err := json.Unmarshal(frame.Data, &t)
				if err != nil {
					a.c.logger.Warn("failed to deserialize termina size", "error", err, "value", string(frame.Data))
					continue
				}
				a.c.logger.Warn("resized terminal", "value", string(frame.Data))

				resizeCh <- t

			}
		}
	}()

	go a.forwardOutput(encoder, outReader, "stdout")
	go a.forwardOutput(encoder, errReader, "stderr")

	h := ar.GetTaskExecHandler(req.Task)
	r, err := h(ctx, drivers.ExecOptions{
		Command: req.Cmd,
		Tty:     req.Tty,
	}, inReader, outWriter, errWriter, resizeCh)

	a.c.logger.Debug("taskExec Handler finished", "result", r, "error", err)

	sendFrame := func(frame *sframer.StreamFrame) {
		buf := new(bytes.Buffer)
		frameCodec := codec.NewEncoder(buf, structs.JsonHandle)
		frameCodec.MustEncode(frame)

		encoder.Encode(cstructs.StreamErrWrapper{
			Payload: buf.Bytes(),
		})
	}

	if err != nil {
		sendFrame(&sframer.StreamFrame{
			Data:      []byte(err.Error()),
			FileEvent: "exit-error",
		})
		return
	}

	sendFrame(&sframer.StreamFrame{
		Data:      []byte(strconv.Itoa(r.ExitCode)),
		FileEvent: "exit-code",
	})

	a.c.logger.Info("exec exited", "result", r, "err", err)

	return
}

func (a *Allocations) forwardOutput(encoder *codec.Encoder,
	reader io.Reader, source string) error {

	buf := new(bytes.Buffer)
	frameCodec := codec.NewEncoder(buf, structs.JsonHandle)
	frame := &sframer.StreamFrame{
		File: source,
	}

	bytes := make([]byte, 1024)

	for {
		n, err := reader.Read(bytes)
		if err == io.EOF || err == io.ErrClosedPipe {
			return nil
		} else if err != nil {
			a.c.logger.Warn("failed to read exec output", "source", source, "error", err)
		}

		frame.Data = bytes[:n]
		frameCodec.MustEncode(frame)

		encoder.Encode(cstructs.StreamErrWrapper{
			Payload: buf.Bytes(),
		})

		buf.Reset()
		frameCodec.Reset(buf)
	}
}
