package structs

import (
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/mapstructure"
)

const (
	VolumeTypeHost = "host"
)

// ClientHostVolumeConfig is used to configure access to host paths on a Nomad Client
type ClientHostVolumeConfig struct {
	Name     string `hcl:",key"`
	Path     string `hcl:"path"`
	ReadOnly bool   `hcl:"read_only"`
	Hidden   bool   `hcl:"hidden"`
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

// HostVolumeConfig is the struct that is expected inside the `config` section
// of a `host` type volume.
type HostVolumeConfig struct {
	// Source is the name of the desired HostVolume.
	Source string
}

func (h *HostVolumeConfig) Copy() *HostVolumeConfig {
	if h == nil {
		return nil
	}
	nh := new(HostVolumeConfig)
	*nh = *h
	return nh
}

// VolumeRequest is a representation of a storage volume that a TaskGroup wishes to use.
type VolumeRequest struct {
	Name     string
	Type     string
	ReadOnly bool
	Hidden   bool

	Config map[string]interface{}
}

func (v *VolumeRequest) Copy() *VolumeRequest {
	if v == nil {
		return nil
	}
	nv := new(VolumeRequest)
	*nv = *v

	if i, err := copystructure.Copy(nv.Config); err != nil {
		panic(err.Error())
	} else {
		nv.Config = i.(map[string]interface{})
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

// VolumeMount is ...
type VolumeMount struct {
	Volume      string
	Destination string
	ReadOnly    bool
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

func ParseHostVolumeConfig(m map[string]interface{}) (*HostVolumeConfig, error) {
	var c HostVolumeConfig
	err := mapstructure.Decode(m, &c)

	return &c, err
}
