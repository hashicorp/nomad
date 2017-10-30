package agent

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/api"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/client"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

const (
	agentHttpCheckInterval  = 10 * time.Second
	agentHttpCheckTimeout   = 5 * time.Second
	serverRpcCheckInterval  = 10 * time.Second
	serverRpcCheckTimeout   = 3 * time.Second
	serverSerfCheckInterval = 10 * time.Second
	serverSerfCheckTimeout  = 3 * time.Second

	// roles used in identifying Consul entries for Nomad agents
	consulRoleServer = "server"
	consulRoleClient = "client"
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

	// consulService is Nomad's custom Consul client for managing services
	// and checks.
	consulService *consul.ServiceClient

	// consulCatalog is the subset of Consul's Catalog API Nomad uses.
	consulCatalog consul.CatalogAPI

	// consulSupportsTLSSkipVerify flags whether or not Nomad can register
	// checks with TLSSkipVerify
	consulSupportsTLSSkipVerify bool

	client *client.Client

	server *nomad.Server

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	InmemSink *metrics.InmemSink
}

// NewAgent is used to create a new agent with the given configuration
func NewAgent(config *Config, logOutput io.Writer, inmem *metrics.InmemSink) (*Agent, error) {
	a := &Agent{
		config:     config,
		logger:     log.New(logOutput, "", log.LstdFlags|log.Lmicroseconds),
		logOutput:  logOutput,
		shutdownCh: make(chan struct{}),
		InmemSink:  inmem,
	}

	if err := a.setupConsul(config.Consul); err != nil {
		return nil, fmt.Errorf("Failed to initialize Consul client: %v", err)
	}
	if err := a.setupServer(); err != nil {
		return nil, err
	}
	if err := a.setupClient(); err != nil {
		return nil, err
	}
	if a.client == nil && a.server == nil {
		return nil, fmt.Errorf("must have at least client or server mode enabled")
	}

	return a, nil
}

// convertServerConfig takes an agent config and log output and returns a Nomad
// Config.
func convertServerConfig(agentConfig *Config, logOutput io.Writer) (*nomad.Config, error) {
	conf := agentConfig.NomadConfig
	if conf == nil {
		conf = nomad.DefaultConfig()
	}
	conf.LogOutput = logOutput
	conf.DevMode = agentConfig.DevMode
	conf.Build = agentConfig.Version.VersionNumber()
	if agentConfig.Region != "" {
		conf.Region = agentConfig.Region
	}

	// Set the Authoritative Region if set, otherwise default to
	// the same as the local region.
	if agentConfig.Server.AuthoritativeRegion != "" {
		conf.AuthoritativeRegion = agentConfig.Server.AuthoritativeRegion
	} else if agentConfig.Region != "" {
		conf.AuthoritativeRegion = agentConfig.Region
	}

	if agentConfig.Datacenter != "" {
		conf.Datacenter = agentConfig.Datacenter
	}
	if agentConfig.NodeName != "" {
		conf.NodeName = agentConfig.NodeName
	}
	if agentConfig.Server.BootstrapExpect > 0 {
		if agentConfig.Server.BootstrapExpect == 1 {
			conf.Bootstrap = true
		} else {
			atomic.StoreInt32(&conf.BootstrapExpect, int32(agentConfig.Server.BootstrapExpect))
		}
	}
	if agentConfig.DataDir != "" {
		conf.DataDir = filepath.Join(agentConfig.DataDir, "server")
	}
	if agentConfig.Server.DataDir != "" {
		conf.DataDir = agentConfig.Server.DataDir
	}
	if agentConfig.Server.ProtocolVersion != 0 {
		conf.ProtocolVersion = uint8(agentConfig.Server.ProtocolVersion)
	}
	if agentConfig.Server.NumSchedulers != 0 {
		conf.NumSchedulers = agentConfig.Server.NumSchedulers
	}
	if len(agentConfig.Server.EnabledSchedulers) != 0 {
		conf.EnabledSchedulers = agentConfig.Server.EnabledSchedulers
	}
	if agentConfig.ACL.Enabled {
		conf.ACLEnabled = true
	}
	if agentConfig.ACL.ReplicationToken != "" {
		conf.ReplicationToken = agentConfig.ACL.ReplicationToken
	}
	if agentConfig.Sentinel != nil {
		conf.SentinelConfig = agentConfig.Sentinel
	}

	// Set up the bind addresses
	rpcAddr, err := net.ResolveTCPAddr("tcp", agentConfig.normalizedAddrs.RPC)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse RPC address %q: %v", agentConfig.normalizedAddrs.RPC, err)
	}
	serfAddr, err := net.ResolveTCPAddr("tcp", agentConfig.normalizedAddrs.Serf)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Serf address %q: %v", agentConfig.normalizedAddrs.Serf, err)
	}
	conf.RPCAddr.Port = rpcAddr.Port
	conf.RPCAddr.IP = rpcAddr.IP
	conf.SerfConfig.MemberlistConfig.BindPort = serfAddr.Port
	conf.SerfConfig.MemberlistConfig.BindAddr = serfAddr.IP.String()

	// Set up the advertise addresses
	rpcAddr, err = net.ResolveTCPAddr("tcp", agentConfig.AdvertiseAddrs.RPC)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse RPC advertise address %q: %v", agentConfig.AdvertiseAddrs.RPC, err)
	}
	serfAddr, err = net.ResolveTCPAddr("tcp", agentConfig.AdvertiseAddrs.Serf)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Serf advertise address %q: %v", agentConfig.AdvertiseAddrs.Serf, err)
	}
	conf.RPCAdvertise = rpcAddr
	conf.SerfConfig.MemberlistConfig.AdvertiseAddr = serfAddr.IP.String()
	conf.SerfConfig.MemberlistConfig.AdvertisePort = serfAddr.Port

	// Set up gc threshold and heartbeat grace period
	if gcThreshold := agentConfig.Server.NodeGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.NodeGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.JobGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.JobGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.EvalGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.EvalGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.DeploymentGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.DeploymentGCThreshold = dur
	}

	if heartbeatGrace := agentConfig.Server.HeartbeatGrace; heartbeatGrace != 0 {
		conf.HeartbeatGrace = heartbeatGrace
	}
	if min := agentConfig.Server.MinHeartbeatTTL; min != 0 {
		conf.MinHeartbeatTTL = min
	}
	if maxHPS := agentConfig.Server.MaxHeartbeatsPerSecond; maxHPS != 0 {
		conf.MaxHeartbeatsPerSecond = maxHPS
	}

	if *agentConfig.Consul.AutoAdvertise && agentConfig.Consul.ServerServiceName == "" {
		return nil, fmt.Errorf("server_service_name must be set when auto_advertise is enabled")
	}

	// Add the Consul and Vault configs
	conf.ConsulConfig = agentConfig.Consul
	conf.VaultConfig = agentConfig.Vault

	// Set the TLS config
	conf.TLSConfig = agentConfig.TLSConfig

	// Setup telemetry related config
	conf.StatsCollectionInterval = agentConfig.Telemetry.collectionInterval

	return conf, nil
}

// serverConfig is used to generate a new server configuration struct
// for initializing a nomad server.
func (a *Agent) serverConfig() (*nomad.Config, error) {
	return convertServerConfig(a.config, a.logOutput)
}

// clientConfig is used to generate a new client configuration struct
// for initializing a Nomad client.
func (a *Agent) clientConfig() (*clientconfig.Config, error) {
	// Setup the configuration
	conf := a.config.ClientConfig
	if conf == nil {
		conf = clientconfig.DefaultConfig()
	}
	if a.server != nil {
		conf.RPCHandler = a.server
	}
	conf.LogOutput = a.logOutput
	conf.LogLevel = a.config.LogLevel
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
	conf.ChrootEnv = a.config.Client.ChrootEnv
	conf.Options = a.config.Client.Options
	// Logging deprecation messages about consul related configuration in client
	// options
	var invalidConsulKeys []string
	for key := range conf.Options {
		if strings.HasPrefix(key, "consul") {
			invalidConsulKeys = append(invalidConsulKeys, fmt.Sprintf("options.%s", key))
		}
	}
	if len(invalidConsulKeys) > 0 {
		a.logger.Printf("[WARN] agent: Invalid keys: %v", strings.Join(invalidConsulKeys, ","))
		a.logger.Printf(`Nomad client ignores consul related configuration in client options.
		Please refer to the guide https://www.nomadproject.io/docs/agent/configuration/consul.html
		to configure Nomad to work with Consul.`)
	}

	if a.config.Client.NetworkSpeed != 0 {
		conf.NetworkSpeed = a.config.Client.NetworkSpeed
	}
	if a.config.Client.CpuCompute != 0 {
		conf.CpuCompute = a.config.Client.CpuCompute
	}
	if a.config.Client.MaxKillTimeout != "" {
		dur, err := time.ParseDuration(a.config.Client.MaxKillTimeout)
		if err != nil {
			return nil, fmt.Errorf("Error parsing max kill timeout: %s", err)
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

	// Set up the HTTP advertise address
	conf.Node.HTTPAddr = a.config.AdvertiseAddrs.HTTP

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

	conf.Version = a.config.Version

	if *a.config.Consul.AutoAdvertise && a.config.Consul.ClientServiceName == "" {
		return nil, fmt.Errorf("client_service_name must be set when auto_advertise is enabled")
	}

	conf.ConsulConfig = a.config.Consul
	conf.VaultConfig = a.config.Vault

	// Set up Telemetry configuration
	conf.StatsCollectionInterval = a.config.Telemetry.collectionInterval
	conf.PublishNodeMetrics = a.config.Telemetry.PublishNodeMetrics
	conf.PublishAllocationMetrics = a.config.Telemetry.PublishAllocationMetrics
	conf.DisableTaggedMetrics = a.config.Telemetry.DisableTaggedMetrics
	conf.BackwardsCompatibleMetrics = a.config.Telemetry.BackwardsCompatibleMetrics

	// Set the TLS related configs
	conf.TLSConfig = a.config.TLSConfig
	conf.Node.TLSEnabled = conf.TLSConfig.EnableHTTP

	// Set the GC related configs
	conf.GCInterval = a.config.Client.GCInterval
	conf.GCParallelDestroys = a.config.Client.GCParallelDestroys
	conf.GCDiskUsageThreshold = a.config.Client.GCDiskUsageThreshold
	conf.GCInodeUsageThreshold = a.config.Client.GCInodeUsageThreshold
	conf.GCMaxAllocs = a.config.Client.GCMaxAllocs
	if a.config.Client.NoHostUUID != nil {
		conf.NoHostUUID = *a.config.Client.NoHostUUID
	} else {
		// Default no_host_uuid to true
		conf.NoHostUUID = true
	}

	// Setup the ACLs
	conf.ACLEnabled = a.config.ACL.Enabled
	conf.ACLTokenTTL = a.config.ACL.TokenTTL
	conf.ACLPolicyTTL = a.config.ACL.PolicyTTL

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

	// Sets up the keyring for gossip encryption
	if err := a.setupKeyrings(conf); err != nil {
		return fmt.Errorf("failed to configure keyring: %v", err)
	}

	// Create the server
	server, err := nomad.NewServer(conf, a.consulCatalog, a.logger)
	if err != nil {
		return fmt.Errorf("server setup failed: %v", err)
	}
	a.server = server

	// Consul check addresses default to bind but can be toggled to use advertise
	rpcCheckAddr := a.config.normalizedAddrs.RPC
	serfCheckAddr := a.config.normalizedAddrs.Serf
	if *a.config.Consul.ChecksUseAdvertise {
		rpcCheckAddr = a.config.AdvertiseAddrs.RPC
		serfCheckAddr = a.config.AdvertiseAddrs.Serf
	}

	// Create the Nomad Server services for Consul
	if *a.config.Consul.AutoAdvertise {
		httpServ := &structs.Service{
			Name:      a.config.Consul.ServerServiceName,
			PortLabel: a.config.AdvertiseAddrs.HTTP,
			Tags:      []string{consul.ServiceTagHTTP},
		}
		const isServer = true
		if check := a.agentHTTPCheck(isServer); check != nil {
			httpServ.Checks = []*structs.ServiceCheck{check}
		}
		rpcServ := &structs.Service{
			Name:      a.config.Consul.ServerServiceName,
			PortLabel: a.config.AdvertiseAddrs.RPC,
			Tags:      []string{consul.ServiceTagRPC},
			Checks: []*structs.ServiceCheck{
				{
					Name:      "Nomad Server RPC Check",
					Type:      "tcp",
					Interval:  serverRpcCheckInterval,
					Timeout:   serverRpcCheckTimeout,
					PortLabel: rpcCheckAddr,
				},
			},
		}
		serfServ := &structs.Service{
			Name:      a.config.Consul.ServerServiceName,
			PortLabel: a.config.AdvertiseAddrs.Serf,
			Tags:      []string{consul.ServiceTagSerf},
			Checks: []*structs.ServiceCheck{
				{
					Name:      "Nomad Server Serf Check",
					Type:      "tcp",
					Interval:  serverSerfCheckInterval,
					Timeout:   serverSerfCheckTimeout,
					PortLabel: serfCheckAddr,
				},
			},
		}

		// Add the http port check if TLS isn't enabled
		consulServices := []*structs.Service{
			rpcServ,
			serfServ,
			httpServ,
		}
		if err := a.consulService.RegisterAgent(consulRoleServer, consulServices); err != nil {
			return err
		}
	}

	return nil
}

// setupKeyrings is used to initialize and load keyrings during agent startup
func (a *Agent) setupKeyrings(config *nomad.Config) error {
	file := filepath.Join(a.config.DataDir, serfKeyring)

	if a.config.Server.EncryptKey == "" {
		goto LOAD
	}
	if _, err := os.Stat(file); err != nil {
		if err := initKeyring(file, a.config.Server.EncryptKey); err != nil {
			return err
		}
	}

LOAD:
	if _, err := os.Stat(file); err == nil {
		config.SerfConfig.KeyringFile = file
	}
	if err := loadKeyringFile(config.SerfConfig); err != nil {
		return err
	}
	// Success!
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

	client, err := client.NewClient(conf, a.consulCatalog, a.consulService, a.logger)
	if err != nil {
		return fmt.Errorf("client setup failed: %v", err)
	}
	a.client = client

	// Create the Nomad Client  services for Consul
	if *a.config.Consul.AutoAdvertise {
		httpServ := &structs.Service{
			Name:      a.config.Consul.ClientServiceName,
			PortLabel: a.config.AdvertiseAddrs.HTTP,
			Tags:      []string{consul.ServiceTagHTTP},
		}
		const isServer = false
		if check := a.agentHTTPCheck(isServer); check != nil {
			httpServ.Checks = []*structs.ServiceCheck{check}
		}
		if err := a.consulService.RegisterAgent(consulRoleClient, []*structs.Service{httpServ}); err != nil {
			return err
		}
	}

	return nil
}

// agentHTTPCheck returns a health check for the agent's HTTP API if possible.
// If no HTTP health check can be supported nil is returned.
func (a *Agent) agentHTTPCheck(server bool) *structs.ServiceCheck {
	// Resolve the http check address
	httpCheckAddr := a.config.normalizedAddrs.HTTP
	if *a.config.Consul.ChecksUseAdvertise {
		httpCheckAddr = a.config.AdvertiseAddrs.HTTP
	}
	check := structs.ServiceCheck{
		Name:      "Nomad Client HTTP Check",
		Type:      "http",
		Path:      "/v1/agent/health?type=client",
		Protocol:  "http",
		Interval:  agentHttpCheckInterval,
		Timeout:   agentHttpCheckTimeout,
		PortLabel: httpCheckAddr,
	}
	// Switch to endpoint that doesn't require a leader for servers
	if server {
		check.Name = "Nomad Server HTTP Check"
		check.Path = "/v1/agent/health?type=server"
	}
	if !a.config.TLSConfig.EnableHTTP {
		// No HTTPS, return a plain http check
		return &check
	}
	if !a.consulSupportsTLSSkipVerify {
		a.logger.Printf("[WARN] agent: not registering Nomad HTTPS Health Check because it requires Consul>=0.7.2")
		return nil
	}
	if a.config.TLSConfig.VerifyHTTPSClient {
		a.logger.Printf("[WARN] agent: not registering Nomad HTTPS Health Check because verify_https_client enabled")
		return nil
	}

	// HTTPS enabled; skip verification
	check.Protocol = "https"
	check.TLSSkipVerify = true
	return &check
}

// reservePortsForClient reserves a range of ports for the client to use when
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

	if err := a.consulService.Shutdown(); err != nil {
		a.logger.Printf("[ERR] agent: shutting down Consul client failed: %v", err)
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

// setupConsul creates the Consul client and starts its main Run loop.
func (a *Agent) setupConsul(consulConfig *config.ConsulConfig) error {
	apiConf, err := consulConfig.ApiConfig()
	if err != nil {
		return err
	}
	client, err := api.NewClient(apiConf)
	if err != nil {
		return err
	}

	// Determine version for TLSSkipVerify
	if self, err := client.Agent().Self(); err == nil {
		a.consulSupportsTLSSkipVerify = consulSupportsTLSSkipVerify(self)
	}

	// Create Consul Catalog client for service discovery.
	a.consulCatalog = client.Catalog()

	// Create Consul Service client for service advertisement and checks.
	a.consulService = consul.NewServiceClient(client.Agent(), a.consulSupportsTLSSkipVerify, a.logger)

	// Run the Consul service client's sync'ing main loop
	go a.consulService.Run()
	return nil
}

var consulTLSSkipVerifyMinVersion = version.Must(version.NewVersion("0.7.2"))

// consulSupportsTLSSkipVerify returns true if Consul supports TLSSkipVerify.
func consulSupportsTLSSkipVerify(self map[string]map[string]interface{}) bool {
	member, ok := self["Member"]
	if !ok {
		return false
	}
	tagsI, ok := member["Tags"]
	if !ok {
		return false
	}
	tags, ok := tagsI.(map[string]interface{})
	if !ok {
		return false
	}
	buildI, ok := tags["build"]
	if !ok {
		return false
	}
	build, ok := buildI.(string)
	if !ok {
		return false
	}
	parts := strings.SplitN(build, ":", 2)
	if len(parts) != 2 {
		return false
	}
	v, err := version.NewVersion(parts[0])
	if err != nil {
		return false
	}
	if v.LessThan(consulTLSSkipVerifyMinVersion) {
		return false
	}
	return true
}
