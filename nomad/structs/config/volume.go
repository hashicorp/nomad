package config

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
