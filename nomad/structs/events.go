package structs

// Subsystem denotes the subsystem where a node event took place.
type Subsystem string

const (
	Drain        Subsystem = "Drain"
	Driver       Subsystem = "Driver"
	Heartbeating Subsystem = "Heartbeating"
)

// NodeEvent is a single unit representing a node’s state change
type NodeEvent struct {
	Message string
	Subsystem
	Details   map[string]string
	Timestamp int64
}
