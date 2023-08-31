package integrations

// RenewalRequest is a request object for renewal of both tokens and secret's
// leases.
type RenewalRequest struct {
	// ErrCh is the channel into which any renewal error will be sent to
	ErrCh chan error

	// ID is an identifier which represents either a token or a lease
	ID string

	// Increment is the duration for which the token or lease should be
	// renewed for
	Increment int

	// IsToken indicates whether the 'id' field is a token or not
	IsToken bool
}
