package api

// Allocs is used to query the alloc-related endpoints.
type Allocs struct {
	client *Client
}

// Allocs returns a handle on the allocs endpoints.
func (c *Client) Allocs() *Allocs {
	return &Allocs{client: c}
}

// List returns a list of all of the allocations.
func (a *Allocs) List() ([]*Alloc, *QueryMeta, error) {
	var resp []*Alloc
	qm, err := a.client.query("/v1/allocations", &resp, nil)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Alloc is used for serialization of allocations.
type Alloc struct {
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
