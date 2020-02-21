package api

// Scaling is used to query scaling-related API endpoints
type Scaling struct {
	client *Client
}

// Scaling returns a handle on the scaling endpoints.
func (c *Client) Scaling() *Scaling {
	return &Scaling{client: c}
}

func (s *Scaling) ListPolicies(q *QueryOptions) ([]*ScalingPolicyListStub, *QueryMeta, error) {
	var resp []*ScalingPolicyListStub
	qm, err := s.client.query("/v1/scaling/policies", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

func (s *Scaling) GetPolicy(ID string, q *QueryOptions) (*ScalingPolicy, *QueryMeta, error) {
	var policy ScalingPolicy
	qm, err := s.client.query("/v1/scaling/policy/"+ID, &policy, q)
	if err != nil {
		return nil, nil, err
	}
	return &policy, qm, nil
}

func (p *ScalingPolicy) Canonicalize() {
	if p.Enabled == nil {
		p.Enabled = boolToPtr(true)
	}
}

// ScalingRequest is the payload for a generic scaling action
type ScalingRequest struct {
	Value  interface{}
	Reason string
	Error  string
	Meta   map[string]interface{}
	WriteRequest
	// this is effectively a job update, so we need the ability to override policy.
	PolicyOverride bool
}

// ScalingPolicy is the user-specified API object for an autoscaling policy
type ScalingPolicy struct {
	ID          string
	Namespace   string
	Target      map[string]string
	Min         int64
	Max         int64
	Policy      map[string]interface{}
	Enabled     *bool
	CreateIndex uint64
	ModifyIndex uint64
}

// ScalingPolicyListStub is used to return a subset of scaling policy information
// for the scaling policy list
type ScalingPolicyListStub struct {
	ID          string
	Enabled     bool
	Target      map[string]string
	CreateIndex uint64
	ModifyIndex uint64
}

// ScaleStatusResponse is the payload for a generic scaling action
type ScaleStatusResponse struct {
	JobID          string
	JobModifyIndex uint64
	Value          interface{}
}
