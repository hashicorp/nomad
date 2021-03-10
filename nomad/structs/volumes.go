package structs

const (
	VolumeTypeHost = "host"
)

const (
	VolumeMountPropagationPrivate       = "private"
	VolumeMountPropagationHostToTask    = "host-to-task"
	VolumeMountPropagationBidirectional = "bidirectional"
)

func MountPropagationModeIsValid(propagationMode string) bool {
	switch propagationMode {
	case "", VolumeMountPropagationPrivate, VolumeMountPropagationHostToTask, VolumeMountPropagationBidirectional:
		return true
	default:
		return false
	}
}

// ClientHostVolumeConfig is used to configure access to host paths on a Nomad Client
type ClientHostVolumeConfig struct {
	Name     string `hcl:",key"`
	Path     string `hcl:"path"`
	ReadOnly bool   `hcl:"read_only"`
}

func (p *ClientHostVolumeConfig) Copy() *ClientHostVolumeConfig {
	if p == nil {
		return nil
	}

	c := new(ClientHostVolumeConfig)
	*c = *p
	return c
}

func CopyMapStringClientHostVolumeConfig(m map[string]*ClientHostVolumeConfig) map[string]*ClientHostVolumeConfig {
	if m == nil {
		return nil
	}

	nm := make(map[string]*ClientHostVolumeConfig, len(m))
	for k, v := range m {
		nm[k] = v.Copy()
	}

	return nm
}

func CopySliceClientHostVolumeConfig(s []*ClientHostVolumeConfig) []*ClientHostVolumeConfig {
	l := len(s)
	if l == 0 {
		return nil
	}

	ns := make([]*ClientHostVolumeConfig, l)
	for idx, cfg := range s {
		ns[idx] = cfg.Copy()
	}

	return ns
}

func HostVolumeSliceMerge(a, b []*ClientHostVolumeConfig) []*ClientHostVolumeConfig {
	n := make([]*ClientHostVolumeConfig, len(a))
	seenKeys := make(map[string]int, len(a))

	for i, config := range a {
		n[i] = config.Copy()
		seenKeys[config.Name] = i
	}

	for _, config := range b {
		if fIndex, ok := seenKeys[config.Name]; ok {
			n[fIndex] = config.Copy()
			continue
		}

		n = append(n, config.Copy())
	}

	return n
}

// VolumeRequest is a representation of a storage volume that a TaskGroup wishes to use.
type VolumeRequest struct {
	Name         string
	Type         string
	Source       string
	ReadOnly     bool
	MountOptions *CSIMountOptions
	PerAlloc     bool
}

func (v *VolumeRequest) Copy() *VolumeRequest {
	if v == nil {
		return nil
	}
	nv := new(VolumeRequest)
	*nv = *v

	if v.MountOptions != nil {
		nv.MountOptions = v.MountOptions.Copy()
	}

	return nv
}

func CopyMapVolumeRequest(s map[string]*VolumeRequest) map[string]*VolumeRequest {
	if s == nil {
		return nil
	}

	l := len(s)
	c := make(map[string]*VolumeRequest, l)
	for k, v := range s {
		c[k] = v.Copy()
	}
	return c
}

// VolumeMount represents the relationship between a destination path in a task
// and the task group volume that should be mounted there.
type VolumeMount struct {
	Volume          string
	Destination     string
	ReadOnly        bool
	PropagationMode string
}

func (v *VolumeMount) Copy() *VolumeMount {
	if v == nil {
		return nil
	}

	nv := new(VolumeMount)
	*nv = *v
	return nv
}

func CopySliceVolumeMount(s []*VolumeMount) []*VolumeMount {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*VolumeMount, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}
