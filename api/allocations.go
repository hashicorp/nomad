package api

// Allocations is used to query the alloc-related endpoints.
type Allocations struct {
	client *Client
}

// Allocations returns a handle on the allocs endpoints.
func (c *Client) Allocations() *Allocations {
	return &Allocations{client: c}
}

// List returns a list of all of the allocations.
func (a *Allocations) List() ([]*Allocation, *QueryMeta, error) {
	var resp []*Allocation
	qm, err := a.client.query("/v1/allocations", &resp, nil)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Allocation is used for serialization of allocations.
type Allocation struct {
	ID                 string
	EvalID             string
	Name               string
	NodeID             string
	JobID              string
	TaskGroup          string
	DesiredStatus      string
	DesiredDescription string
	ClientStatus       string
	ClientDescription  string
}
