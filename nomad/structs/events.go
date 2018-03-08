package structs

// Subsystem denotes the subsystem where a node event took place.
type Subsystem string

const (
	Drain     Subsystem = "Drain"
	Driver    Subsystem = "Driver"
	Heartbeat Subsystem = "Heartbeat"
	Server    Subsystem = "Server"
	Cluster   Subsystem = "Cluster"
)

// NodeEvent is a single unit representing a node’s state change
type NodeEvent struct {
	Message   string
	Subsystem Subsystem
	Details   map[string]string
	Timestamp int64

	CreateIndex uint64
}

// EmitNodeEventRequest is a client request to update the node events source
// with a new client-side event
type EmitNodeEventRequest struct {
	NodeID    string
	NodeEvent *NodeEvent
	WriteRequest
}

// EmitNodeEventResponse is a server response to the client about the status of
// the node event source update.
type EmitNodeEventResponse struct {
	Index uint64
	WriteMeta
}
