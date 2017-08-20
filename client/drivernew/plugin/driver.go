package plugin

// Driver is the interface that must be met to be a Nomad driver.
type Driver interface {
	// Name returns the name of the client
	Name() (string, error)

	// Exit is used to exit the Driver process
	Exit() error
}
