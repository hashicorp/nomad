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
	Source   string `hcl:"source"`
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

func HostVolumeSliceMerge(a, b []*ClientHostVolumeConfig) []*ClientHostVolumeConfig {
	n := make([]*ClientHostVolumeConfig, len(a))
	seenKeys := make(map[string]struct{}, len(a))

	for k, v := range a {
		if _, ok := seenKeys[v.Name]; ok {
			continue
		}
		n[k] = v.Copy()
		seenKeys[v.Name] = struct{}{}
	}
	for k, v := range b {
		if _, ok := seenKeys[v.Name]; ok {
			continue
		}
		n[k] = v.Copy()
		seenKeys[v.Name] = struct{}{}
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

// Volume is a representation of a storage volume that a TaskGroup wishes to use.
type Volume struct {
	Name     string
	Type     string
	ReadOnly bool
	Hidden   bool

	Config map[string]interface{}
}

func (v *Volume) Copy() *Volume {
	if v == nil {
		return nil
	}
	nv := new(Volume)
	*nv = *v

	if i, err := copystructure.Copy(nv.Config); err != nil {
		panic(err.Error())
	} else {
		nv.Config = i.(map[string]interface{})
	}

	return nv
}

func CopyMapVolumes(s map[string]*Volume) map[string]*Volume {
	if s == nil {
		return nil
	}

	l := len(s)
	c := make(map[string]*Volume, l)
	for k, v := range s {
		c[k] = v.Copy()
	}
	return c
}

// VolumeMount is ...
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

type VolumeRequest struct {
	Volume *Volume
	Config map[string]interface{}
}

func (h *VolumeRequest) Copy() *VolumeRequest {
	if h == nil {
		return nil
	}

	c := new(VolumeRequest)
	c.Volume = h.Volume.Copy()
	if i, err := copystructure.Copy(h.Config); err != nil {
		panic(err.Error())
	} else {
		c.Config = i.(map[string]interface{})
	}
	return c
}

func CopyMapVolumeRequest(m map[string]*VolumeRequest) map[string]*VolumeRequest {
	if m == nil {
		return nil
	}

	l := len(m)
	c := make(map[string]*VolumeRequest, l)
	for k, v := range m {
		c[k] = v.Copy()
	}
	return c
}

func ParseHostVolumeConfig(m map[string]interface{}) (*HostVolumeConfig, error) {
	var c HostVolumeConfig
	err := mapstructure.Decode(m, &c)

	return &c, err
}
