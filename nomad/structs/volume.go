package structs

import "time"

// Volume is a representation of a persistent storage volume from an external
// provider
type Volume struct {
	ID       string
	Provider string
	ReadOnly bool
}

func (v *Volume) Copy() *Volume {
	if v == nil {
		return nil
	}
	nv := new(Volume)
	*nv = *v
	return nv
}

func CopySliceVolumes(s []*Volume) []*Volume {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*Volume, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}

// StorageInfo is the current state of a single storage provider. This is
// updated regularly as provider health changes on the node.
type StorageInfo struct {
	Attributes        map[string]string
	Detected          bool
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time
}
