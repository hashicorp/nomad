package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/snappy"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-msgpack/codec"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	allocNotFoundErr    = "allocation not found"
	resourceNotFoundErr = "resource not found"
)

func (s *HTTPServer) AllocsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.AllocListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	// Parse resources and task_states field selection
	resources, err := parseBool(req, "resources")
	if err != nil {
		return nil, err
	}
	taskStates, err := parseBool(req, "task_states")
	if err != nil {
		return nil, err
	}

	if resources != nil || taskStates != nil {
		args.Fields = structs.NewAllocStubFields()
		if resources != nil {
			args.Fields.Resources = *resources
		}
		if taskStates != nil {
			args.Fields.TaskStates = *taskStates
		}
	}

	var out structs.AllocListResponse
	if err := s.agent.RPC("Alloc.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Allocations == nil {
		out.Allocations = make([]*structs.AllocListStub, 0)
	}
	for _, alloc := range out.Allocations {
		alloc.SetEventDisplayMessages()
	}
	return out.Allocations, nil
}

func (s *HTTPServer) AllocSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/allocation/")

	// tokenize the suffix of the path to get the alloc id and find the action
	// invoked on the alloc id
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) > 2 || len(tokens) < 1 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	allocID := tokens[0]

	if len(tokens) == 1 {
		return s.allocGet(allocID, resp, req)
	}

	switch tokens[1] {
	case "stop":
		return s.allocStop(allocID, resp, req)
	}

	return nil, CodedError(404, resourceNotFoundErr)
}

func (s *HTTPServer) allocGet(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.AllocSpecificRequest{
		AllocID: allocID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleAllocResponse
	if err := s.agent.RPC("Alloc.GetAlloc", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Alloc == nil {
		return nil, CodedError(404, "alloc not found")
	}

	// Decode the payload if there is any

	alloc := out.Alloc
	if alloc.Job != nil && len(alloc.Job.Payload) != 0 {
		decoded, err := snappy.Decode(nil, alloc.Job.Payload)
		if err != nil {
			return nil, err
		}
		alloc = alloc.Copy()
		alloc.Job.Payload = decoded
	}
	alloc.SetEventDisplayMessages()

	// Handle 0.12 ports upgrade path
	alloc = alloc.Copy()
	alloc.AllocatedResources.Canonicalize()

	return alloc, nil
}

func (s *HTTPServer) allocStop(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if !(req.Method == "POST" || req.Method == "PUT") {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	noShutdownDelay := false
	if noShutdownDelayQS := req.URL.Query().Get("no_shutdown_delay"); noShutdownDelayQS != "" {
		var err error
		noShutdownDelay, err = strconv.ParseBool(noShutdownDelayQS)
		if err != nil {
			return nil, fmt.Errorf("no_shutdown_delay value is not a boolean: %v", err)
		}
	}

	sr := &structs.AllocStopRequest{
		AllocID:         allocID,
		NoShutdownDelay: noShutdownDelay,
	}
	s.parseWriteRequest(req, &sr.WriteRequest)

	var out structs.AllocStopResponse
	rpcErr := s.agent.RPC("Alloc.Stop", &sr, &out)

	if rpcErr != nil {
		if structs.IsErrUnknownAllocation(rpcErr) {
			rpcErr = CodedError(404, allocNotFoundErr)
		}
		return nil, rpcErr
	}

	setIndex(resp, out.Index)
	return &out, nil
}

func (s *HTTPServer) ClientAllocRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/client/allocation/")

	// tokenize the suffix of the path to get the alloc id and find the action
	// invoked on the alloc id
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) != 2 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	allocID := tokens[0]
	switch tokens[1] {
	case "stats":
		return s.allocStats(allocID, resp, req)
	case "exec":
		return s.allocExec(allocID, resp, req)
	case "snapshot":
		if s.agent.client == nil {
			return nil, clientNotRunning
		}
		return s.allocSnapshot(allocID, resp, req)
	case "restart":
		return s.allocRestart(allocID, resp, req)
	case "gc":
		return s.allocGC(allocID, resp, req)
	case "signal":
		return s.allocSignal(allocID, resp, req)
	}

	return nil, CodedError(404, resourceNotFoundErr)
}

func (s *HTTPServer) ClientGCRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the requested Node ID
	requestedNode := req.URL.Query().Get("node_id")

	// Build the request and parse the ACL token
	args := structs.NodeSpecificRequest{
		NodeID: requestedNode,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(requestedNode)

	// Make the RPC
	var reply structs.GenericResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("Allocations.GarbageCollectAll", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientAllocations.GarbageCollectAll", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientAllocations.GarbageCollectAll", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return nil, rpcErr
}

func (s *HTTPServer) allocRestart(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Build the request and parse the ACL token
	args := structs.AllocRestartRequest{
		AllocID:  allocID,
		TaskName: "",
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Explicitly parse the body separately to disallow overriding AllocID in req Body.
	var reqBody struct {
		TaskName string
	}
	err := json.NewDecoder(req.Body).Decode(&reqBody)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if reqBody.TaskName != "" {
		args.TaskName = reqBody.TaskName
	}

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForAlloc(allocID)

	// Make the RPC
	var reply structs.GenericResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("Allocations.Restart", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientAllocations.Restart", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientAllocations.Restart", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) || structs.IsErrUnknownAllocation(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return reply, rpcErr
}

func (s *HTTPServer) allocGC(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Build the request and parse the ACL token
	args := structs.AllocSpecificRequest{
		AllocID: allocID,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForAlloc(allocID)

	// Make the RPC
	var reply structs.GenericResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("Allocations.GarbageCollect", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientAllocations.GarbageCollect", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientAllocations.GarbageCollect", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) || structs.IsErrUnknownAllocation(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return nil, rpcErr
}

func (s *HTTPServer) allocSignal(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if !(req.Method == "POST" || req.Method == "PUT") {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Build the request and parse the ACL token
	args := structs.AllocSignalRequest{}
	err := decodeBody(req, &args)
	if err != nil {
		return nil, CodedError(400, fmt.Sprintf("Failed to decode body: %v", err))
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
	args.AllocID = allocID

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForAlloc(allocID)

	// Make the RPC
	var reply structs.GenericResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("Allocations.Signal", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientAllocations.Signal", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientAllocations.Signal", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) || structs.IsErrUnknownAllocation(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return reply, rpcErr
}

func (s *HTTPServer) allocSnapshot(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var secret string
	s.parseToken(req, &secret)
	if !s.agent.Client().ValidateMigrateToken(allocID, secret) {
		return nil, structs.ErrPermissionDenied
	}

	allocFS, err := s.agent.Client().GetAllocFS(allocID)
	if err != nil {
		return nil, fmt.Errorf(allocNotFoundErr)
	}
	if err := allocFS.Snapshot(resp); err != nil {
		return nil, fmt.Errorf("error making snapshot: %v", err)
	}
	return nil, nil
}

func (s *HTTPServer) allocStats(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	// Build the request and parse the ACL token
	task := req.URL.Query().Get("task")
	args := cstructs.AllocStatsRequest{
		AllocID: allocID,
		Task:    task,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Determine the handler to use
	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForAlloc(allocID)

	// Make the RPC
	var reply cstructs.AllocStatsResponse
	var rpcErr error
	if useLocalClient {
		rpcErr = s.agent.Client().ClientRPC("Allocations.Stats", &args, &reply)
	} else if useClientRPC {
		rpcErr = s.agent.Client().RPC("ClientAllocations.Stats", &args, &reply)
	} else if useServerRPC {
		rpcErr = s.agent.Server().RPC("ClientAllocations.Stats", &args, &reply)
	} else {
		rpcErr = CodedError(400, "No local Node and node_id not provided")
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) || structs.IsErrUnknownAllocation(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}
	}

	return reply.Stats, rpcErr
}

func (s *HTTPServer) allocExec(allocID string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Build the request and parse the ACL token
	task := req.URL.Query().Get("task")
	cmdJsonStr := req.URL.Query().Get("command")
	var command []string
	err := json.Unmarshal([]byte(cmdJsonStr), &command)
	if err != nil {
		// this shouldn't happen, []string is always be serializable to json
		return nil, fmt.Errorf("failed to marshal command into json: %v", err)
	}

	ttyB := false
	if tty := req.URL.Query().Get("tty"); tty != "" {
		ttyB, err = strconv.ParseBool(tty)
		if err != nil {
			return nil, fmt.Errorf("tty value is not a boolean: %v", err)
		}
	}

	args := cstructs.AllocExecRequest{
		AllocID: allocID,
		Task:    task,
		Cmd:     command,
		Tty:     ttyB,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	conn, err := s.wsUpgrader.Upgrade(resp, req, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade connection: %v", err)
	}

	if err := readWsHandshake(conn.ReadJSON, req, &args.QueryOptions); err != nil {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(toWsCode(400), err.Error()))
		return nil, err
	}

	return s.execStreamImpl(conn, &args)
}

// readWsHandshake reads the websocket handshake message and sets
// query authentication token, if request requires a handshake
func readWsHandshake(readFn func(interface{}) error, req *http.Request, q *structs.QueryOptions) error {

	// Avoid handshake if request doesn't require one
	if hv := req.URL.Query().Get("ws_handshake"); hv == "" {
		return nil
	} else if h, err := strconv.ParseBool(hv); err != nil {
		return fmt.Errorf("ws_handshake value is not a boolean: %v", err)
	} else if !h {
		return nil
	}

	var h wsHandshakeMessage
	err := readFn(&h)
	if err != nil {
		return err
	}

	supportedWSHandshakeVersion := 1
	if h.Version != supportedWSHandshakeVersion {
		return fmt.Errorf("unexpected handshake value: %v", h.Version)
	}

	q.AuthToken = h.AuthToken
	return nil
}

type wsHandshakeMessage struct {
	Version   int    `json:"version"`
	AuthToken string `json:"auth_token"`
}

func (s *HTTPServer) execStreamImpl(ws *websocket.Conn, args *cstructs.AllocExecRequest) (interface{}, error) {
	allocID := args.AllocID
	method := "Allocations.Exec"

	// Get the correct handler
	localClient, remoteClient, localServer := s.rpcHandlerForAlloc(allocID)
	var handler structs.StreamingRpcHandler
	var handlerErr error
	if localClient {
		handler, handlerErr = s.agent.Client().StreamingRpcHandler(method)
	} else if remoteClient {
		handler, handlerErr = s.agent.Client().RemoteStreamingRpcHandler(method)
	} else if localServer {
		handler, handlerErr = s.agent.Server().StreamingRpcHandler(method)
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}

	// Create a pipe connecting the (possibly remote) handler to the http response
	httpPipe, handlerPipe := net.Pipe()
	decoder := codec.NewDecoder(httpPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(httpPipe, structs.MsgpackHandle)

	// Create a goroutine that closes the pipe if the connection closes.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		httpPipe.Close()

		// don't close ws - wait to drain messages
	}()

	// Create a channel that decodes the results
	errCh := make(chan HTTPCodedError, 2)

	// stream response
	go func() {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		go forwardExecInput(encoder, ws, errCh)

		for {
			var res cstructs.StreamErrWrapper
			err := decoder.Decode(&res)
			if isClosedError(err) {
				ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				errCh <- nil
				return
			}

			if err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
			decoder.Reset(httpPipe)

			if err := res.Error; err != nil {
				code := 500
				if err.Code != nil {
					code = int(*err.Code)
				}
				errCh <- CodedError(code, err.Error())
				return
			}

			if err := ws.WriteMessage(websocket.TextMessage, res.Payload); err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
		}
	}()

	// start streaming request to streaming RPC - returns when streaming completes or errors
	handler(handlerPipe)
	// stop streaming background goroutines for streaming - but not websocket activity
	cancel()
	// retrieve any error and/or wait until goroutine stop and close errCh connection before
	// closing websocket connection
	codedErr := <-errCh

	// we won't return an error on ws close, but at least make it available in
	// the logs so we can trace spurious disconnects
	s.logger.Debug("alloc exec channel closed with error", "error", codedErr)

	if isClosedError(codedErr) {
		codedErr = nil
	} else if codedErr != nil {
		ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(toWsCode(codedErr.Code()), codedErr.Error()))
	}
	ws.Close()

	return nil, codedErr
}

func toWsCode(httpCode int) int {
	switch httpCode {
	case 500:
		return websocket.CloseInternalServerErr
	default:
		// placeholder error code
		return websocket.ClosePolicyViolation
	}
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}

	return err == io.EOF ||
		err == io.ErrClosedPipe ||
		strings.Contains(err.Error(), "closed") ||
		strings.Contains(err.Error(), "EOF")
}

// forwardExecInput forwards exec input (e.g. stdin) from websocket connection
// to the streaming RPC connection to client
func forwardExecInput(encoder *codec.Encoder, ws *websocket.Conn, errCh chan<- HTTPCodedError) {
	for {
		sf := &drivers.ExecTaskStreamingRequestMsg{}
		err := ws.ReadJSON(sf)
		if err == io.EOF {
			return
		}

		if err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		err = encoder.Encode(sf)
		if err != nil {
			errCh <- CodedError(500, err.Error())
		}
	}
}
