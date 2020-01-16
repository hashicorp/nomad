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

// ScalingRequeset is the payload for a generic scaling action
type ScalingRequest struct {
	JobID  string
	Value  interface{}
	Reason string
	WriteRequest
	PolicyOverride bool
}
