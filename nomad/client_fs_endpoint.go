package nomad

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
)

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

	// Decode the arguments
	var args cstructs.FsLogsRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		f.handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	// TODO
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	//args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	//if done, err := f.srv.forward("FileSystem.Logs", args, args, reply); done {
	//return err
	//}
	defer metrics.MeasureSince([]string{"nomad", "file_system", "logs"}, time.Now())

	// Check node read permissions
	if aclObj, err := f.srv.ResolveToken(args.AuthToken); err != nil {
		//return err
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

	// Get the connection to the client
	state, ok := f.srv.getNodeConn(nodeID)
	if !ok {
		// Determine the Server that has a connection to the node.
		//srv, err := f.srv.serverWithNodeConn(nodeID)
		//if err != nil {
		//f.handleStreamResultError(err, nil, encoder)
		//return
		//}

		// TODO Forward streaming
		//return s.srv.forwardServer(srv, "ClientStats.Stats", args, reply)
		return
	}

	stream, err := NodeStreamingRpc(state.Session, "FileSystem.Logs")
	if err != nil {
		f.handleStreamResultError(err, nil, encoder)
		return
	}
	defer stream.Close()

	// Send the request.
	outEncoder := codec.NewEncoder(stream, structs.MsgpackHandle)
	if err := outEncoder.Encode(args); err != nil {
		f.handleStreamResultError(err, nil, encoder)
		return
	}

	Bridge(conn, stream)
	return
}
