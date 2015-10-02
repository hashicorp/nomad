package discovery

// Discovery provides a generic interface which can be used to implement
// service discovery back-ends in Nomad.
type Discovery interface {
	// Register is used to register a new entry into a service discovery
	// system. The address and port are the location of the service, and
	// the name is the symbolic name used to query it. The node is the
	// unique node ID known to Nomad.
	Register(node, name, address string, port int) error

	// Deregister is used to deregister a service from a discovery system.
	Deregister(node, name string) error
}
