package structs

import (
	"github.com/mitchellh/copystructure"
)

const (
	VolumeTypeHost = "host"
)

// ClientHostVolumeConfig is used to configure access to host paths on a Nomad Client
type ClientHostVolumeConfig struct {
	Name     string
	Type     string
	Source   string
	ReadOnly bool `mapstructure:"read_only"`
	Hidden   bool
}

func (p *ClientHostVolumeConfig) Copy() *ClientHostVolumeConfig {
	if p == nil {
		return nil
	}

	c := new(ClientHostVolumeConfig)
	*c = *p
	return c
}

func HostVolumeSetMerge(a, b map[string]*ClientHostVolumeConfig) map[string]*ClientHostVolumeConfig {
	n := make(map[string]*ClientHostVolumeConfig, len(a))
	for k, v := range a {
		n[k] = v.Copy()
	}
	for k, v := range b {
		n[k] = v.Copy()
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

type HostVolumeRequest struct {
	Volume *Volume
	Config *HostVolumeConfig
}

func (h *HostVolumeRequest) Copy() *HostVolumeRequest {
	if h == nil {
		return nil
	}

	nh := new(HostVolumeRequest)
	nh.Volume = h.Volume.Copy()
	nh.Config = h.Config.Copy()
	return nh
}

func CopyMapHostVolumeRequest(m map[string]*HostVolumeRequest) map[string]*HostVolumeRequest {
	if m == nil {
		return nil
	}

	l := len(m)
	c := make(map[string]*HostVolumeRequest, l)
	for k, v := range m {
		c[k] = v.Copy()
	}
	return c
}
