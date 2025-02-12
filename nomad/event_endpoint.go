// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"io"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Event struct {
	srv *Server
}

func NewEventEndpoint(srv *Server) *Event {
	return &Event{srv: srv}
}

func (e *Event) register() {
	e.srv.streamingRpcs.Register("Event.Stream", e.stream)
}

func (e *Event) stream(conn io.ReadWriteCloser) {
	defer conn.Close()

	var args structs.EventStreamRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleJsonResultError(err, pointer.Of(int64(500)), encoder)
		return
	}

	authErr := e.srv.Authenticate(nil, &args)
	if authErr != nil {
		handleJsonResultError(structs.ErrPermissionDenied, pointer.Of(int64(403)), encoder)
		return
	}

	// forward to appropriate region
	if args.Region != e.srv.config.Region {
		err := e.forwardStreamingRPC(args.Region, "Event.Stream", args, conn)
		if err != nil {
			handleJsonResultError(err, pointer.Of(int64(500)), encoder)
		}
		return
	}

	e.srv.MeasureRPCRate("event", structs.RateMetricRead, &args)

	resolvedACL, err := e.srv.ResolveACL(&args)
	if err != nil {
		handleJsonResultError(structs.ErrPermissionDenied, pointer.Of(int64(403)), encoder)
		return
	}

	validatedNses, err := e.validateACL(args.Namespace, args.Topics, resolvedACL)
	if err != nil {
		handleJsonResultError(structs.ErrPermissionDenied, pointer.Of(int64(403)), encoder)
		return
	}

	// Generate the subscription request
	subReq := &stream.SubscribeRequest{
		Token:  args.AuthToken,
		Topics: args.Topics,
		Index:  uint64(args.Index),
		// Namespaces is set once, in the event a users ACL is updated to include
		// more NSes, the current event stream will not include the new NSes.
		Namespaces: validatedNses,
		Authenticate: func() error {
			if err := e.srv.Authenticate(nil, &args); err != nil {
				return err
			}
			resolvedACL, err := e.srv.ResolveACL(&args)
			if err != nil {
				return err
			}
			_, err = e.validateACL(args.Namespace, args.Topics, resolvedACL)
			return err
		},
	}

	// Get the servers broker and subscribe
	publisher, err := e.srv.State().EventBroker()
	if err != nil {
		handleJsonResultError(err, pointer.Of(int64(500)), encoder)
		return
	}

	// start subscription to publisher
	var subscription *stream.Subscription
	var subErr error

	subscription, subErr = publisher.Subscribe(subReq)
	if subErr != nil {
		handleJsonResultError(subErr, pointer.Of(int64(500)), encoder)
		return
	}
	defer subscription.Unsubscribe()

	// because we have authenticated, the identity will be set, so extract expiration time
	var exp time.Time
	if c := args.GetIdentity().GetClaims(); c != nil {
		exp = c.Expiry.Time()
	} else if t := args.GetIdentity().GetACLToken(); t != nil && t.ExpirationTime != nil {
		exp = *t.ExpirationTime
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// goroutine to detect remote side closing
	go func() {
		io.Copy(io.Discard, conn)
		cancel()
	}()

	jsonStream := stream.NewJsonStream(ctx, 30*time.Second)
	errCh := make(chan error)
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

			// Ensure the token being used is not expired before we send any events
			// to subscribers.
			if !exp.IsZero() && exp.Before(time.Now().UTC()) {
				select {
				case errCh <- structs.ErrTokenExpired:
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
		handleJsonResultError(streamErr, pointer.Of(int64(500)), encoder)
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

// validateACL handles wildcard namespaces by replacing it with all existing namespaces
// and validates the user has the appropriate ACL to read topics in each one.
func (e *Event) validateACL(namespace string, topics map[structs.Topic][]string, resolvedAcl *acl.ACL) ([]string, error) {
	nses := []string{}
	if namespace == structs.AllNamespacesSentinel {
		ns, _ := e.srv.State().NamespaceNames()
		nses = append(nses, ns...)
	} else {
		nses = append(nses, namespace)
	}

	for _, ns := range nses {
		if err := validateNsOp(ns, topics, resolvedAcl); err != nil {
			return nil, err
		}
	}
	return nses, nil
}

func validateNsOp(namespace string, topics map[structs.Topic][]string, aclObj *acl.ACL) error {
	for topic := range topics {
		switch topic {
		case structs.TopicDeployment,
			structs.TopicEvaluation,
			structs.TopicAllocation,
			structs.TopicJob,
			structs.TopicService:
			if ok := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob); !ok {
				return structs.ErrPermissionDenied
			}
		case structs.TopicHostVolume:
			if ok := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityHostVolumeRead); !ok {
				return structs.ErrPermissionDenied
			}
		case structs.TopicCSIVolume:
			if ok := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityCSIReadVolume); !ok {
				return structs.ErrPermissionDenied
			}
		case structs.TopicCSIPlugin:
			if ok := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob); !ok {
				return structs.ErrPermissionDenied
			}
		case structs.TopicNode:
			if ok := aclObj.AllowNodeRead(); !ok {
				return structs.ErrPermissionDenied
			}
		case structs.TopicNodePool:
			// Require management token for node pools since we can't filter
			// out node pools the token doesn't have access to.
			if ok := aclObj.IsManagement(); !ok {
				return structs.ErrPermissionDenied
			}
		default:
			if ok := aclObj.IsManagement(); !ok {
				return structs.ErrPermissionDenied
			}
		}
	}

	return nil

}
