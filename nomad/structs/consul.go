package structs

import (
	"fmt"
	"reflect"
)

type ConsulConnect struct {
	Native         bool
	SidecarService *ConsulSidecarService
}

func (c *ConsulConnect) Copy() *ConsulConnect {
	if c == nil {
		return nil
	}

	return &ConsulConnect{
		Native:         c.Native,
		SidecarService: c.SidecarService.Copy(),
	}
}

func (c *ConsulConnect) Equals(o *ConsulConnect) bool {
	if c == nil || o == nil {
		return c == o
	}

	if c.Native != o.Native {
		return false
	}

	return c.SidecarService.Equals(o.SidecarService)
}

func (c *ConsulConnect) HasSidecar() bool {
	return c != nil && c.SidecarService != nil
}

func (c *ConsulConnect) Validate() error {
	if c.Native && c.SidecarService != nil {
		return fmt.Errorf("Consul Connect must be native or use a sidecar service; not both")
	}

	return nil
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
	Config              map[string]interface{} // JSON proxy config
}

func (p *ConsulProxy) Copy() *ConsulProxy {
	if p == nil {
		return nil
	}

	upstreams := make([]*ConsulUpstream, len(p.Upstreams))

	for i := range p.Upstreams {
		upstreams[i] = p.Upstreams[i].Copy()
	}

	newP := &ConsulProxy{
		Upstreams: upstreams,
		Config:    make(map[string]interface{}, len(p.Config)),
	}

	for k, v := range p.Config {
		newP.Config[k] = v
	}

	return p
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

	// Avoid nil vs {} differences
	if len(p.Config) != 0 && len(o.Config) != 0 {
		if !reflect.DeepEqual(p.Config, o.Config) {
			return false
		}
	}

	return true
}

type ConsulUpstream struct {
	DestinationName string
	LocalBindPort   int
}

func (u *ConsulUpstream) Copy() *ConsulUpstream {
	if u == nil {
		return nil
	}

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
