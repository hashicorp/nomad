package structs

// HostVolume is a representation of a storage volume from the host
type HostVolume struct {
	Name     string
	Path     string
	ReadOnly bool
}

func (v *HostVolume) Copy() *HostVolume {
	if v == nil {
		return nil
	}
	nv := new(HostVolume)
	*nv = *v
	return nv
}

func CopySliceVolumes(s []*HostVolume) []*HostVolume {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*HostVolume, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}

// VolumeMount is a representation of the configuration required to mount a Volume
// into a Task
type VolumeMount struct {
	VolumeName string
	MountPath  string
	ReadOnly   bool
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
