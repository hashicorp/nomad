package nomad

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
)

// TODO a Single RPC for "Cat", "ReadAt", "Stream" endpoints
// TODO all the non-streaming RPC endpoints

// FileSystem endpoint is used for accessing the logs and filesystem of
// allocations from a Node.
type FileSystem struct {
	srv *Server
}

func (f *FileSystem) Register() {
	f.srv.streamingRpcs.Register("FileSystem.Logs", f.Logs)
}

func (f *FileSystem) handleStreamResultError(err error, code *int64, encoder *codec.Encoder) {
	// Nothing to do as the conn is closed
	if err == io.EOF || strings.Contains(err.Error(), "closed") {
		return
	}

	// Attempt to send the error
	encoder.Encode(&cstructs.StreamErrWrapper{
		Error: cstructs.NewRpcError(err, code),
	})
}

// Stats is used to retrieve the Clients stats.
func (f *FileSystem) Logs(conn io.ReadWriteCloser) {
	defer conn.Close()
	defer metrics.MeasureSince([]string{"nomad", "file_system", "logs"}, time.Now())

	// Decode the arguments
	var args cstructs.FsLogsRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		f.handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	// Check if we need to forward to a different region
	if r := args.RequestRegion(); r != f.srv.Region() {
		// Request the allocation from the target region
		allocReq := &structs.AllocSpecificRequest{
			AllocID:      args.AllocID,
			QueryOptions: args.QueryOptions,
		}
		var allocResp structs.SingleAllocResponse
		if err := f.srv.forwardRegion(r, "Alloc.GetAlloc", allocReq, &allocResp); err != nil {
			f.handleStreamResultError(err, nil, encoder)
			return
		}

		if allocResp.Alloc == nil {
			f.handleStreamResultError(fmt.Errorf("unknown allocation %q", args.AllocID), nil, encoder)
			return
		}

		// Determine the Server that has a connection to the node.
		srv, err := f.srv.serverWithNodeConn(allocResp.Alloc.NodeID, r)
		if err != nil {
			f.handleStreamResultError(err, nil, encoder)
			return
		}

		// Get a connection to the server
		srvConn, err := f.srv.streamingRpc(srv, "FileSystem.Logs")
		if err != nil {
			f.handleStreamResultError(err, nil, encoder)
			return
		}
		defer srvConn.Close()

		// Send the request.
		outEncoder := codec.NewEncoder(srvConn, structs.MsgpackHandle)
		if err := outEncoder.Encode(args); err != nil {
			f.handleStreamResultError(err, nil, encoder)
			return
		}

		Bridge(conn, srvConn)
		return

	}

	// Check node read permissions
	if aclObj, err := f.srv.ResolveToken(args.AuthToken); err != nil {
		f.handleStreamResultError(err, nil, encoder)
		return
	} else if aclObj != nil {
		readfs := aclObj.AllowNsOp(args.QueryOptions.Namespace, acl.NamespaceCapabilityReadFS)
		logs := aclObj.AllowNsOp(args.QueryOptions.Namespace, acl.NamespaceCapabilityReadLogs)
		if !readfs && !logs {
			f.handleStreamResultError(structs.ErrPermissionDenied, nil, encoder)
			return
		}
	}

	// Verify the arguments.
	if args.AllocID == "" {
		f.handleStreamResultError(errors.New("missing AllocID"), helper.Int64ToPtr(400), encoder)
		return
	}

	// Retrieve the allocation
	snap, err := f.srv.State().Snapshot()
	if err != nil {
		f.handleStreamResultError(err, nil, encoder)
		return
	}

	alloc, err := snap.AllocByID(nil, args.AllocID)
	if err != nil {
		f.handleStreamResultError(err, nil, encoder)
		return
	}
	if alloc == nil {
		f.handleStreamResultError(fmt.Errorf("unknown alloc ID %q", args.AllocID), helper.Int64ToPtr(404), encoder)
		return
	}
	nodeID := alloc.NodeID

	// Get the connection to the client either by forwarding to another server
	// or creating a direct stream
	var clientConn net.Conn
	state, ok := f.srv.getNodeConn(nodeID)
	if !ok {
		// Determine the Server that has a connection to the node.
		srv, err := f.srv.serverWithNodeConn(nodeID, f.srv.Region())
		if err != nil {
			f.handleStreamResultError(err, nil, encoder)
			return
		}

		// Get a connection to the server
		conn, err := f.srv.streamingRpc(srv, "FileSystem.Logs")
		if err != nil {
			f.handleStreamResultError(err, nil, encoder)
			return
		}

		clientConn = conn
	} else {
		stream, err := NodeStreamingRpc(state.Session, "FileSystem.Logs")
		if err != nil {
			f.handleStreamResultError(err, nil, encoder)
			return
		}
		clientConn = stream
	}
	defer clientConn.Close()

	// Send the request.
	outEncoder := codec.NewEncoder(clientConn, structs.MsgpackHandle)
	if err := outEncoder.Encode(args); err != nil {
		f.handleStreamResultError(err, nil, encoder)
		return
	}

	Bridge(conn, clientConn)
	return
}
