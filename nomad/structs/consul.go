package structs

type ConsulConnect struct {
	SidecarService *ConsulSidecarService
}

func (c *ConsulConnect) Copy() *ConsulConnect {
	return &ConsulConnect{
		SidecarService: c.SidecarService.Copy(),
	}
}

func (c *ConsulConnect) Equals(o *ConsulConnect) bool {
	if c == nil || o == nil {
		return c == o
	}

	return c.SidecarService.Equals(o.SidecarService)
}

func (c *ConsulConnect) HasSidecar() bool {
	return c != nil && c.SidecarService != nil
}

type ConsulSidecarService struct {
	Port  string
	Proxy *ConsulProxy
}

func (s *ConsulSidecarService) Copy() *ConsulSidecarService {
	return &ConsulSidecarService{
		Port:  s.Port,
		Proxy: s.Proxy.Copy(),
	}
}

func (s *ConsulSidecarService) Equals(o *ConsulSidecarService) bool {
	if s == nil || o == nil {
		return s == o
	}

	if s.Port != o.Port {
		return false
	}

	return s.Proxy.Equals(o.Proxy)
}

type ConsulProxy struct {
	LocalServiceAddress string //TODO(schmichael) expose via api?!
	LocalServicePort    int    //TODO(schmichael) expose via api?!
	Upstreams           []*ConsulUpstream
}

func (p *ConsulProxy) Copy() *ConsulProxy {
	upstreams := make([]*ConsulUpstream, len(p.Upstreams))

	for i := range p.Upstreams {
		upstreams[i] = p.Upstreams[i].Copy()
	}

	return &ConsulProxy{
		Upstreams: upstreams,
	}
}

func (p *ConsulProxy) Equals(o *ConsulProxy) bool {
	if p == nil || o == nil {
		return p == o
	}

	if p.LocalServiceAddress != o.LocalServiceAddress {
		return false
	}

	if p.LocalServicePort != o.LocalServicePort {
		return false
	}

	if len(p.Upstreams) != len(o.Upstreams) {
		return false
	}

	// Order doesn't matter
OUTER:
	for _, up := range p.Upstreams {
		for _, innerUp := range o.Upstreams {
			if up.Equals(innerUp) {
				// Match; find next upstream
				continue OUTER
			}
		}

		// No match
		return false
	}

	return true
}

type ConsulUpstream struct {
	DestinationName string
	LocalBindPort   int
}

func (u *ConsulUpstream) Copy() *ConsulUpstream {
	return &ConsulUpstream{
		DestinationName: u.DestinationName,
		LocalBindPort:   u.LocalBindPort,
	}
}

func (u *ConsulUpstream) Equals(o *ConsulUpstream) bool {
	if u == nil || o == nil {
		return u == o
	}

	return (*u) == (*o)
}
