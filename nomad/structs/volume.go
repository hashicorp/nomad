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
