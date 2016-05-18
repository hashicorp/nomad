package agent

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Agent is a long running daemon that is used to run both
// clients and servers. Servers are responsible for managing
// state and making scheduling decisions. Clients can be
// scheduled to, and are responsible for interfacing with
// servers to run allocations.
type Agent struct {
	config    *Config
	logger    *log.Logger
	logOutput io.Writer

	consulService  *consul.ConsulService // consulService registers the Nomad agent with the consul agent
	consulConfig   *consul.ConsulConfig  // consulConfig is the consul configuration the Nomad client uses to connect with Consul agent
	serverHTTPAddr string
	clientHTTPAddr string

	server *nomad.Server
	client *client.Client

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewAgent is used to create a new agent with the given configuration
func NewAgent(config *Config, logOutput io.Writer) (*Agent, error) {
	// Ensure we have a log sink
	if logOutput == nil {
		logOutput = os.Stderr
	}

	a := &Agent{
		config:     config,
		logger:     log.New(logOutput, "", log.LstdFlags),
		logOutput:  logOutput,
		shutdownCh: make(chan struct{}),
	}

	// creating the consul client configuration that both the server and client
	// uses
	a.createConsulConfig()

	if err := a.setupServer(); err != nil {
		return nil, err
	}
	if err := a.setupClient(); err != nil {
		return nil, err
	}
	if a.client == nil && a.server == nil {
		return nil, fmt.Errorf("must have at least client or server mode enabled")
	}
	if a.config.ConsulConfig.AutoRegister {
		if err := a.syncAgentServicesWithConsul(a.serverHTTPAddr, a.clientHTTPAddr); err != nil {
			a.logger.Printf("[ERR] agent: unable to sync agent services with consul: %v", err)
		}
		if a.consulService != nil {
			go a.consulService.PeriodicSync()
		}
	}
	return a, nil
}

// serverConfig is used to generate a new server configuration struct
// for initializing a nomad server.
func (a *Agent) serverConfig() (*nomad.Config, error) {
	conf := a.config.NomadConfig
	if conf == nil {
		conf = nomad.DefaultConfig()
	}
	conf.LogOutput = a.logOutput
	conf.DevMode = a.config.DevMode
	conf.Build = fmt.Sprintf("%s%s", a.config.Version, a.config.VersionPrerelease)
	if a.config.Region != "" {
		conf.Region = a.config.Region
	}
	if a.config.Datacenter != "" {
		conf.Datacenter = a.config.Datacenter
	}
	if a.config.NodeName != "" {
		conf.NodeName = a.config.NodeName
	}
	if a.config.Server.BootstrapExpect > 0 {
		if a.config.Server.BootstrapExpect == 1 {
			conf.Bootstrap = true
		} else {
			conf.BootstrapExpect = a.config.Server.BootstrapExpect
		}
	}
	if a.config.DataDir != "" {
		conf.DataDir = filepath.Join(a.config.DataDir, "server")
	}
	if a.config.Server.DataDir != "" {
		conf.DataDir = a.config.Server.DataDir
	}
	if a.config.Server.ProtocolVersion != 0 {
		conf.ProtocolVersion = uint8(a.config.Server.ProtocolVersion)
	}
	if a.config.Server.NumSchedulers != 0 {
		conf.NumSchedulers = a.config.Server.NumSchedulers
	}
	if len(a.config.Server.EnabledSchedulers) != 0 {
		conf.EnabledSchedulers = a.config.Server.EnabledSchedulers
	}

	// Set up the advertise addrs
	if addr := a.config.AdvertiseAddrs.Serf; addr != "" {
		serfAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("error resolving serf advertise address: %s", err)
		}
		conf.SerfConfig.MemberlistConfig.AdvertiseAddr = serfAddr.IP.String()
		conf.SerfConfig.MemberlistConfig.AdvertisePort = serfAddr.Port
	}
	if addr := a.config.AdvertiseAddrs.RPC; addr != "" {
		rpcAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("error resolving rpc advertise address: %s", err)
		}
		conf.RPCAdvertise = rpcAddr
	}

	// Set up the bind addresses
	if addr := a.config.BindAddr; addr != "" {
		conf.RPCAddr.IP = net.ParseIP(addr)
		conf.SerfConfig.MemberlistConfig.BindAddr = addr
	}
	if addr := a.config.Addresses.RPC; addr != "" {
		conf.RPCAddr.IP = net.ParseIP(addr)
	}

	if addr := a.config.Addresses.Serf; addr != "" {
		conf.SerfConfig.MemberlistConfig.BindAddr = addr
	}

	// Set up the ports
	if port := a.config.Ports.RPC; port != 0 {
		conf.RPCAddr.Port = port
	}
	if port := a.config.Ports.Serf; port != 0 {
		conf.SerfConfig.MemberlistConfig.BindPort = port
	}
	a.serverHTTPAddr = fmt.Sprintf("%v:%v", a.config.Addresses.HTTP, a.config.Ports.HTTP)
	if a.config.AdvertiseAddrs.HTTP != "" {
		a.serverHTTPAddr = a.config.AdvertiseAddrs.HTTP
	}

	if gcThreshold := a.config.Server.NodeGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.NodeGCThreshold = dur
	}

	if heartbeatGrace := a.config.Server.HeartbeatGrace; heartbeatGrace != "" {
		dur, err := time.ParseDuration(heartbeatGrace)
		if err != nil {
			return nil, err
		}
		conf.HeartbeatGrace = dur
	}

	return conf, nil
}

// clientConfig is used to generate a new client configuration struct
// for initializing a nomad client.
func (a *Agent) clientConfig() (*clientconfig.Config, error) {
	// Setup the configuration
	conf := a.config.ClientConfig
	if conf == nil {
		conf = client.DefaultConfig()
	}
	if a.server != nil {
		conf.RPCHandler = a.server
	}
	conf.LogOutput = a.logOutput
	conf.DevMode = a.config.DevMode
	if a.config.Region != "" {
		conf.Region = a.config.Region
	}
	if a.config.DataDir != "" {
		conf.StateDir = filepath.Join(a.config.DataDir, "client")
		conf.AllocDir = filepath.Join(a.config.DataDir, "alloc")
	}
	if a.config.Client.StateDir != "" {
		conf.StateDir = a.config.Client.StateDir
	}
	if a.config.Client.AllocDir != "" {
		conf.AllocDir = a.config.Client.AllocDir
	}
	conf.Servers = a.config.Client.Servers
	if a.config.Client.NetworkInterface != "" {
		conf.NetworkInterface = a.config.Client.NetworkInterface
	}
	conf.Options = a.config.Client.Options
	if a.config.Client.NetworkSpeed != 0 {
		conf.NetworkSpeed = a.config.Client.NetworkSpeed
	}
	if a.config.Client.MaxKillTimeout != "" {
		dur, err := time.ParseDuration(a.config.Client.MaxKillTimeout)
		if err != nil {
			return nil, fmt.Errorf("Error parsing retry interval: %s", err)
		}
		conf.MaxKillTimeout = dur
	}
	conf.ClientMaxPort = uint(a.config.Client.ClientMaxPort)
	conf.ClientMinPort = uint(a.config.Client.ClientMinPort)

	// Setup the node
	conf.Node = new(structs.Node)
	conf.Node.Datacenter = a.config.Datacenter
	conf.Node.Name = a.config.NodeName
	conf.Node.Meta = a.config.Client.Meta
	conf.Node.NodeClass = a.config.Client.NodeClass

	// Setting the proper HTTP Addr
	httpAddr := fmt.Sprintf("%s:%d", a.config.BindAddr, a.config.Ports.HTTP)
	if a.config.Addresses.HTTP != "" && a.config.AdvertiseAddrs.HTTP == "" {
		httpAddr = fmt.Sprintf("%s:%d", a.config.Addresses.HTTP, a.config.Ports.HTTP)
		if _, err := net.ResolveTCPAddr("tcp", httpAddr); err != nil {
			return nil, fmt.Errorf("error resolving http addr: %v:", err)
		}
	} else if a.config.AdvertiseAddrs.HTTP != "" {
		addr, err := net.ResolveTCPAddr("tcp", a.config.AdvertiseAddrs.HTTP)
		if err != nil {
			return nil, fmt.Errorf("error resolving advertise http addr: %v", err)
		}
		httpAddr = fmt.Sprintf("%s:%d", addr.IP.String(), addr.Port)
	}
	conf.Node.HTTPAddr = httpAddr
	a.clientHTTPAddr = httpAddr

	// Reserve resources on the node.
	r := conf.Node.Reserved
	if r == nil {
		r = new(structs.Resources)
		conf.Node.Reserved = r
	}
	r.CPU = a.config.Client.Reserved.CPU
	r.MemoryMB = a.config.Client.Reserved.MemoryMB
	r.DiskMB = a.config.Client.Reserved.DiskMB
	r.IOPS = a.config.Client.Reserved.IOPS
	conf.GloballyReservedPorts = a.config.Client.Reserved.ParsedReservedPorts

	conf.Version = fmt.Sprintf("%s%s", a.config.Version, a.config.VersionPrerelease)
	conf.Revision = a.config.Revision

	conf.ConsulConfig = a.consulConfig

	return conf, nil
}

// setupServer is used to setup the server if enabled
func (a *Agent) setupServer() error {
	if !a.config.Server.Enabled {
		return nil
	}

	// Setup the configuration
	conf, err := a.serverConfig()
	if err != nil {
		return fmt.Errorf("server config setup failed: %s", err)
	}

	// Create the server
	server, err := nomad.NewServer(conf)
	if err != nil {
		return fmt.Errorf("server setup failed: %v", err)
	}

	a.server = server
	return nil
}

// setupClient is used to setup the client if enabled
func (a *Agent) setupClient() error {
	if !a.config.Client.Enabled {
		return nil
	}

	// Setup the configuration
	conf, err := a.clientConfig()
	if err != nil {
		return fmt.Errorf("client setup failed: %v", err)
	}

	// Reserve some ports for the plugins if we are on Windows
	if runtime.GOOS == "windows" {
		if err := a.reservePortsForClient(conf); err != nil {
			return err
		}
	}

	// Create the client
	client, err := client.NewClient(conf)
	if err != nil {
		return fmt.Errorf("client setup failed: %v", err)
	}
	a.client = client
	return nil
}

// reservePortsForClient reservers a range of ports for the client to use when
// it creates various plugins for log collection, executors, drivers, etc
func (a *Agent) reservePortsForClient(conf *clientconfig.Config) error {
	// finding the device name for loopback
	deviceName, addr, mask, err := a.findLoopbackDevice()
	if err != nil {
		return fmt.Errorf("error finding the device name for loopback: %v", err)
	}

	// seeing if the user has already reserved some resources on this device
	var nr *structs.NetworkResource
	if conf.Node.Reserved == nil {
		conf.Node.Reserved = &structs.Resources{}
	}
	for _, n := range conf.Node.Reserved.Networks {
		if n.Device == deviceName {
			nr = n
		}
	}
	// If the user hasn't already created the device, we create it
	if nr == nil {
		nr = &structs.NetworkResource{
			Device:        deviceName,
			IP:            addr,
			CIDR:          mask,
			ReservedPorts: make([]structs.Port, 0),
		}
	}
	// appending the port ranges we want to use for the client to the list of
	// reserved ports for this device
	for i := conf.ClientMinPort; i <= conf.ClientMaxPort; i++ {
		nr.ReservedPorts = append(nr.ReservedPorts, structs.Port{Label: fmt.Sprintf("plugin-%d", i), Value: int(i)})
	}
	conf.Node.Reserved.Networks = append(conf.Node.Reserved.Networks, nr)
	return nil
}

// findLoopbackDevice iterates through all the interfaces on a machine and
// returns the ip addr, mask of the loopback device
func (a *Agent) findLoopbackDevice() (string, string, string, error) {
	var ifcs []net.Interface
	var err error
	ifcs, err = net.Interfaces()
	if err != nil {
		return "", "", "", err
	}
	for _, ifc := range ifcs {
		addrs, err := ifc.Addrs()
		if err != nil {
			return "", "", "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.IsLoopback() {
				if ip.To4() == nil {
					continue
				}
				return ifc.Name, ip.String(), addr.String(), nil
			}
		}
	}

	return "", "", "", fmt.Errorf("no loopback devices with IPV4 addr found")
}

// Leave is used gracefully exit. Clients will inform servers
// of their departure so that allocations can be rescheduled.
func (a *Agent) Leave() error {
	if a.client != nil {
		if err := a.client.Leave(); err != nil {
			a.logger.Printf("[ERR] agent: client leave failed: %v", err)
		}
	}
	if a.server != nil {
		if err := a.server.Leave(); err != nil {
			a.logger.Printf("[ERR] agent: server leave failed: %v", err)
		}
	}
	return nil
}

// Shutdown is used to terminate the agent.
func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
	}

	a.logger.Println("[INFO] agent: requesting shutdown")
	if a.client != nil {
		if err := a.client.Shutdown(); err != nil {
			a.logger.Printf("[ERR] agent: client shutdown failed: %v", err)
		}
	}
	if a.server != nil {
		if err := a.server.Shutdown(); err != nil {
			a.logger.Printf("[ERR] agent: server shutdown failed: %v", err)
		}
	}

	if a.consulService != nil {
		if err := a.consulService.Shutdown(); err != nil {
			a.logger.Printf("[ERR] agent: shutting down consul service failed: %v", err)
		}
	}

	a.logger.Println("[INFO] agent: shutdown complete")
	a.shutdown = true
	close(a.shutdownCh)
	return nil
}

// RPC is used to make an RPC call to the Nomad servers
func (a *Agent) RPC(method string, args interface{}, reply interface{}) error {
	if a.server != nil {
		return a.server.RPC(method, args, reply)
	}
	return a.client.RPC(method, args, reply)
}

// Client returns the configured client or nil
func (a *Agent) Client() *client.Client {
	return a.client
}

// Server returns the configured server or nil
func (a *Agent) Server() *nomad.Server {
	return a.server
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (a *Agent) Stats() map[string]map[string]string {
	stats := make(map[string]map[string]string)
	if a.server != nil {
		subStat := a.server.Stats()
		for k, v := range subStat {
			stats[k] = v
		}
	}
	if a.client != nil {
		subStat := a.client.Stats()
		for k, v := range subStat {
			stats[k] = v
		}
	}
	return stats
}

func (a *Agent) createConsulConfig() {
	cfg := &consul.ConsulConfig{
		Addr:      a.config.ConsulConfig.Addr,
		Token:     a.config.ConsulConfig.Token,
		Auth:      a.config.ConsulConfig.Auth,
		EnableSSL: a.config.ConsulConfig.EnableSSL,
		VerifySSL: a.config.ConsulConfig.VerifySSL,
		CAFile:    a.config.ConsulConfig.CAFile,
		CertFile:  a.config.ConsulConfig.CertFile,
		KeyFile:   a.config.ConsulConfig.KeyFile,
	}
	a.consulConfig = cfg
}

// syncAgentServicesWithConsul syncs the client and server services with Consul
func (a *Agent) syncAgentServicesWithConsul(clientHttpAddr string, serverHttpAddr string) error {
	cs, err := consul.NewConsulService(a.consulConfig, a.logger)
	if err != nil {
		return err
	}
	a.consulService = cs
	var services []*structs.Service
	if a.client != nil && a.config.ConsulConfig.ClientServiceName != "" {
		if err != nil {
			return err
		}
		clientService := &structs.Service{
			Name:      a.config.ConsulConfig.ClientServiceName,
			PortLabel: clientHttpAddr,
		}
		services = append(services, clientService)
		cs.SetServiceIdentifier("agent-client")
	}
	if a.server != nil && a.config.ConsulConfig.ServerServiceName != "" {
		serverService := &structs.Service{
			Name:      a.config.ConsulConfig.ServerServiceName,
			PortLabel: serverHttpAddr,
		}
		services = append(services, serverService)
		cs.SetServiceIdentifier("agent-server")
	}

	cs.SetAddrFinder(func(portLabel string) (string, int) {
		host, port, err := net.SplitHostPort(portLabel)
		if err != nil {
			return "", 0
		}

		// if the addr for the service is ":port", then we default to
		// registering the service with ip as the loopback addr
		if host == "" {
			host = "127.0.0.1"
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			return "", 0
		}
		return host, p
	})

	return cs.SyncServices(services)
}
