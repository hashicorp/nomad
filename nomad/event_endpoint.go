package nomad

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Event struct {
	srv *Server
}

func (e *Event) register() {
	e.srv.streamingRpcs.Register("Event.Stream", e.stream)
}

// ListSinks is used to list the event sinks registered in Nomad
func (e *Event) ListSinks(args *structs.EventSinkListRequest, reply *structs.EventSinkListResponse) error {
	if done, err := e.srv.forward("Event.ListSinks", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "event", "list_sinks"}, time.Now())

	if aclObj, err := e.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowOperatorRead() {
		return structs.ErrPermissionDenied
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			iter, err := state.EventSinks(ws)
			if err != nil {
				return err
			}

			var sinks []*structs.EventSink
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}

				sink := raw.(*structs.EventSink)
				sinks = append(sinks, sink)
			}
			reply.Sinks = sinks

			index, err := state.Index("event_sink")
			if err != nil {
				return err
			}

			// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
			// We floor the index at one, since realistically the first write must have a higher index.
			if index == 0 {
				index = 1
			}

			reply.Index = index
			return nil
		},
	}

	return e.srv.blockingRPC(&opts)
}

// UpsertSink is used to create or update an event sink
func (e *Event) UpsertSink(args *structs.EventSinkUpsertRequest, reply *structs.GenericResponse) error {
	if done, err := e.srv.forward("Event.UpsertSink", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "event", "upsert_sink"}, time.Now())

	if aclObj, err := e.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	if err := args.Sink.Validate(); err != nil {
		return err
	}

	// Update via Raft
	_, index, err := e.srv.raftApply(structs.EventSinkUpsertRequestType, args)
	if err != nil {
		return err
	}

	reply.Index = index
	return nil
}

func (e *Event) UpdateSinks(args *structs.EventSinkProgressRequest, reply *structs.GenericResponse) error {
	if done, err := e.srv.forward("Event.UpdateSinks", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "event", "update_sinks"}, time.Now())

	if aclObj, err := e.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Update via Raft
	_, index, err := e.srv.raftApply(structs.BatchEventSinkUpdateProgressType, args)
	if err != nil {
		return err
	}

	reply.Index = index
	return nil
}

// GetSink returns the requested event sink
func (e *Event) GetSink(args *structs.EventSinkSpecificRequest, reply *structs.EventSinkResponse) error {
	if done, err := e.srv.forward("Event.GetSink", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "event", "get_sink"}, time.Now())

	if aclObj, err := e.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowOperatorRead() {
		return structs.ErrPermissionDenied
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			s, err := state.EventSinkByID(ws, args.ID)
			if err != nil {
				return nil
			}

			reply.Sink = s

			index, err := state.Index("event_sink")
			if err != nil {
				return err
			}

			if index == 0 {
				index = 1
			}

			reply.Index = index
			return nil
		},
	}

	return e.srv.blockingRPC(&opts)
}

// DeleteSink deletes an event sink
func (e *Event) DeleteSink(args *structs.EventSinkDeleteRequest, reply *structs.GenericResponse) error {
	if done, err := e.srv.forward("Event.DeleteSink", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "event", "delete_sink"}, time.Now())

	if aclObj, err := e.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Update via Raft
	_, index, err := e.srv.raftApply(structs.EventSinkDeleteRequestType, args)
	if err != nil {
		return err
	}

	reply.Index = index
	return nil
}

func (e *Event) stream(conn io.ReadWriteCloser) {
	defer conn.Close()

	var args structs.EventStreamRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleJsonResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	// forward to appropriate region
	if args.Region != e.srv.config.Region {
		err := e.forwardStreamingRPC(args.Region, "Event.Stream", args, conn)
		if err != nil {
			handleJsonResultError(err, helper.Int64ToPtr(500), encoder)
		}
		return
	}

	aclObj, err := e.srv.ResolveToken(args.AuthToken)
	if err != nil {
		handleJsonResultError(err, nil, encoder)
		return
	}

	subReq := &stream.SubscribeRequest{
		Token:     args.AuthToken,
		Topics:    args.Topics,
		Index:     uint64(args.Index),
		Namespace: args.Namespace,
	}

	// Check required ACL permissions for requested Topics
	if aclObj != nil {
		if err := aclCheckForEvents(subReq, aclObj); err != nil {
			handleJsonResultError(structs.ErrPermissionDenied, helper.Int64ToPtr(403), encoder)
			return
		}
	}

	// Get the servers broker and subscribe
	publisher, err := e.srv.State().EventBroker()
	if err != nil {
		handleJsonResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start subscription to publisher
	subscription, err := publisher.Subscribe(subReq)
	if err != nil {
		handleJsonResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}
	defer subscription.Unsubscribe()

	errCh := make(chan error)

	jsonStream := stream.NewJsonStream(ctx, 30*time.Second)

	// goroutine to detect remote side closing
	go func() {
		io.Copy(ioutil.Discard, conn)
		cancel()
	}()

	go func() {
		defer cancel()
		for {
			events, err := subscription.Next(ctx)
			if err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}
				return
			}

			// Continue if there are no events
			if len(events.Events) == 0 {
				continue
			}

			if err := jsonStream.Send(events); err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	var streamErr error
OUTER:
	for {
		select {
		case streamErr = <-errCh:
			break OUTER
		case <-ctx.Done():
			break OUTER
		case eventJSON, ok := <-jsonStream.OutCh():
			// check if ndjson may have been closed when an error occurred,
			// check once more for an error.
			if !ok {
				select {
				case streamErr = <-errCh:
					// There was a pending error
				default:
				}
				break OUTER
			}

			var resp structs.EventStreamWrapper
			resp.Event = eventJSON

			if err := encoder.Encode(resp); err != nil {
				streamErr = err
				break OUTER
			}
			encoder.Reset(conn)
		}

	}

	if streamErr != nil {
		handleJsonResultError(streamErr, helper.Int64ToPtr(500), encoder)
		return
	}

}

func (e *Event) forwardStreamingRPC(region string, method string, args interface{}, in io.ReadWriteCloser) error {
	server, err := e.srv.findRegionServer(region)
	if err != nil {
		return err
	}

	return e.forwardStreamingRPCToServer(server, method, args, in)
}

func (e *Event) forwardStreamingRPCToServer(server *serverParts, method string, args interface{}, in io.ReadWriteCloser) error {
	srvConn, err := e.srv.streamingRpc(server, method)
	if err != nil {
		return err
	}
	defer srvConn.Close()

	outEncoder := codec.NewEncoder(srvConn, structs.MsgpackHandle)
	if err := outEncoder.Encode(args); err != nil {
		return err
	}

	structs.Bridge(in, srvConn)
	return nil
}

// handleJsonResultError is a helper for sending an error with a potential
// error code. The transmission of the error is ignored if the error has been
// generated by the closing of the underlying transport.
func handleJsonResultError(err error, code *int64, encoder *codec.Encoder) {
	// Nothing to do as the conn is closed
	if err == io.EOF {
		return
	}

	encoder.Encode(&structs.EventStreamWrapper{
		Error: structs.NewRpcError(err, code),
	})
}

func aclCheckForEvents(subReq *stream.SubscribeRequest, aclObj *acl.ACL) error {
	if len(subReq.Topics) == 0 {
		return fmt.Errorf("invalid topic request")
	}

	reqPolicies := make(map[string]struct{})
	var required = struct{}{}

	for topic := range subReq.Topics {
		switch topic {
		case structs.TopicDeployment, structs.TopicEval,
			structs.TopicAlloc, structs.TopicJob:
			if _, ok := reqPolicies[acl.NamespaceCapabilityReadJob]; !ok {
				reqPolicies[acl.NamespaceCapabilityReadJob] = required
			}
		case structs.TopicNode:
			reqPolicies["node-read"] = required
		case structs.TopicAll:
			reqPolicies["management"] = required
		default:
			return fmt.Errorf("unknown topic %s", topic)
		}
	}

	for checks := range reqPolicies {
		switch checks {
		case acl.NamespaceCapabilityReadJob:
			if ok := aclObj.AllowNsOp(subReq.Namespace, acl.NamespaceCapabilityReadJob); !ok {
				return structs.ErrPermissionDenied
			}
		case "node-read":
			if ok := aclObj.AllowNodeRead(); !ok {
				return structs.ErrPermissionDenied
			}
		case "management":
			if ok := aclObj.IsManagement(); !ok {
				return structs.ErrPermissionDenied
			}
		}
	}

	return nil
}
