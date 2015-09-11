package api

// Nodes is used to query node-related API endpoints
type Nodes struct {
	client *Client
}

// Nodes returns a handle on the node endpoints.
func (c *Client) Nodes() *Nodes {
	return &Nodes{client: c}
}

// List is used to list out all of the nodes
func (n *Nodes) List(q *QueryOptions) ([]*Node, *QueryMeta, error) {
	var resp []*Node
	qm, err := n.client.query("/v1/nodes", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Info is used to query a specific node by its ID.
func (n *Nodes) Info(nodeID string, q *QueryOptions) (*Node, *QueryMeta, error) {
	var resp Node
	qm, err := n.client.query("/v1/node/"+nodeID, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// ToggleDrain is used to toggle drain mode on/off for a given node.
func (n *Nodes) ToggleDrain(nodeID string, drain bool, q *WriteOptions) (*WriteMeta, error) {
	// Conver the bool to a string
	var drainArg string
	if drain {
		drainArg = "true"
	} else {
		drainArg = "false"
	}

	// Send the query
	wm, err := n.client.write("/v1/node/"+nodeID+"/drain?enable="+drainArg, nil, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Allocations is used to return the allocations associated with a node.
func (n *Nodes) Allocations(nodeID string, q *QueryOptions) ([]*Allocation, *QueryMeta, error) {
	var resp []*Allocation
	qm, err := n.client.query("/v1/node/"+nodeID+"/allocations", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// ForceEvaluate is used to force-evaluate an existing node.
func (n *Nodes) ForceEvaluate(nodeID string, q *WriteOptions) (string, *WriteMeta, error) {
	var resp nodeEvalResponse
	wm, err := n.client.write("/v1/node/"+nodeID+"/evaluate", nil, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

// Node is used to deserialize a node entry.
type Node struct {
	ID                string
	Datacenter        string
	Name              string
	NodeClass         string
	Drain             bool
	Status            string
	StatusDescription string
	CreateIndex       uint64
	ModifyIndex       uint64
}

// nodeEvalResponse is used to decode a force-eval.
type nodeEvalResponse struct {
	EvalID string
}
