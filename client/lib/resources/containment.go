package resources

// A Containment will cleanup resources created by an executor.
type Containment interface {
	// Apply enables containment on pid.
	Apply(pid int) error

	// Cleanup will purge executor resources like cgroups.
	Cleanup() error

	// GetPIDs will return the processes overseen by the Containment
	GetPIDs() PIDs
}
