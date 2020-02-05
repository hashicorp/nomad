package api

// ScalingPolicy is the user-specified API object for an autoscaling policy
type ScalingPolicy struct {
	Policy  map[string]interface{}
	Enabled *bool
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
	WriteRequest
	// this is effectively a job update, so we need the ability to override policy.
	PolicyOverride bool
}

// ScaleStatusResponse is the payload for a generic scaling action
type ScaleStatusResponse struct {
	JobID          string
	JobModifyIndex uint64
	Value          interface{}
}
