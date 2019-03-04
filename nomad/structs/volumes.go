package structs

import (
	"github.com/mitchellh/copystructure"
)

// HostVolumeConfig is used to configure access to host paths on a Nomad Client
type HostVolumeConfig struct {
	Name     string
	Type     string
	Source   string
	ReadOnly bool `mapstructure:"read_only"`
	Hidden   bool
}

func (p *HostVolumeConfig) Copy() *HostVolumeConfig {
	if p == nil {
		return nil
	}

	c := new(HostVolumeConfig)
	*c = *p
	return c
}

func HostVolumeSetMerge(a, b map[string]*HostVolumeConfig) map[string]*HostVolumeConfig {
	n := make(map[string]*HostVolumeConfig, len(a))
	for k, v := range a {
		n[k] = v.Copy()
	}
	for k, v := range b {
		n[k] = v.Copy()
	}

	return n
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
