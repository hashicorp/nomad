package api

// Periodic is used to access periodic job endpoints
type Periodic struct {
	client *Client
}

// Periodic returns a handle to access periodic job endpoints.
func (c *Client) PeriodicJobs() *Periodic {
	return &Periodic{client: c}
}

// Force spawns a new instance of the periodic job and returns the eval ID
func (p *Periodic) Force(jobID string, q *WriteOptions) (string, *WriteMeta, error) {
	var resp periodicForceResponse
	wm, err := p.client.write("/v1/periodic/"+jobID+"/force", nil, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

// periodicForceResponse is used to deserialize a force response
type periodicForceResponse struct {
	EvalID string
}
