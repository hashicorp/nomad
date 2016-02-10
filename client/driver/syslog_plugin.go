package driver

import (
	"log"
	"net"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/syslog"
	"github.com/hashicorp/nomad/nomad/structs"
)

type SyslogCollectorRPC struct {
	client *rpc.Client
}

type LaunchCollectorArgs struct {
	AddrNet  string
	AddrName string
	Ctx      *syslog.LogCollectorContext
}

func (e *SyslogCollectorRPC) LaunchCollector(addr net.Addr,
	ctx *syslog.LogCollectorContext) (*syslog.SyslogCollectorState, error) {
	var ss *syslog.SyslogCollectorState
	err := e.client.Call("Plugin.LaunchCollector",
		LaunchCollectorArgs{AddrNet: addr.Network(), AddrName: addr.String(), Ctx: ctx}, &ss)
	return ss, err
}

func (e *SyslogCollectorRPC) Exit() error {
	return e.client.Call("Plugin.Exit", new(interface{}), new(interface{}))
}

func (e *SyslogCollectorRPC) UpdateLogConfig(logConfig *structs.LogConfig) error {
	return e.client.Call("Plugin.UpdateLogConfig", logConfig, new(interface{}))
}

type SyslogCollectorRPCServer struct {
	Impl syslog.LogCollector
}

func (s *SyslogCollectorRPCServer) LaunchCollector(args LaunchCollectorArgs,
	resp *syslog.SyslogCollectorState) error {
	addr, _ := net.ResolveTCPAddr(args.AddrNet, args.AddrName)
	ss, err := s.Impl.LaunchCollector(addr, args.Ctx)
	if err != nil {
		*resp = *ss
	}
	return err
}

func (s *SyslogCollectorRPCServer) Exit(args interface{}, resp *interface{}) error {
	return s.Impl.Exit()
}

func (s *SyslogCollectorRPCServer) UpdateLogConfig(logConfig *structs.LogConfig, resp *interface{}) error {
	return s.Impl.UpdateLogConfig(logConfig)
}

type SyslogCollectorPlugin struct {
	logger *log.Logger
	Impl   *SyslogCollectorRPCServer
}

func (p *SyslogCollectorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	if p.Impl == nil {
		p.Impl = &SyslogCollectorRPCServer{Impl: syslog.NewSyslogCollector(p.logger)}
	}
	return p.Impl, nil
}

func (p *SyslogCollectorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &SyslogCollectorRPC{client: c}, nil
}
