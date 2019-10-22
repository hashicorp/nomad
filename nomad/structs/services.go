package structs

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/mitchellh/copystructure"
)

const (
	EnvoyBootstrapPath = "${NOMAD_SECRETS_DIR}/envoy_bootstrap.json"

	ServiceCheckHTTP   = "http"
	ServiceCheckTCP    = "tcp"
	ServiceCheckScript = "script"
	ServiceCheckGRPC   = "grpc"

	// minCheckInterval is the minimum check interval permitted.  Consul
	// currently has its MinInterval set to 1s.  Mirror that here for
	// consistency.
	minCheckInterval = 1 * time.Second

	// minCheckTimeout is the minimum check timeout permitted for Consul
	// script TTL checks.
	minCheckTimeout = 1 * time.Second
)

// ServiceCheck represents the Consul health check.
type ServiceCheck struct {
	Name          string              // Name of the check, defaults to id
	Type          string              // Type of the check - tcp, http, docker and script
	Command       string              // Command is the command to run for script checks
	Args          []string            // Args is a list of arguments for script checks
	Path          string              // path of the health check url for http type check
	Protocol      string              // Protocol to use if check is http, defaults to http
	PortLabel     string              // The port to use for tcp/http checks
	AddressMode   string              // 'host' to use host ip:port or 'driver' to use driver's
	Interval      time.Duration       // Interval of the check
	Timeout       time.Duration       // Timeout of the response from the check before consul fails the check
	InitialStatus string              // Initial status of the check
	TLSSkipVerify bool                // Skip TLS verification when Protocol=https
	Method        string              // HTTP Method to use (GET by default)
	Header        map[string][]string // HTTP Headers for Consul to set when making HTTP checks
	CheckRestart  *CheckRestart       // If and when a task should be restarted based on checks
	GRPCService   string              // Service for GRPC checks
	GRPCUseTLS    bool                // Whether or not to use TLS for GRPC checks
	TaskName      string              // What task to execute this check in
}

// Copy the stanza recursively. Returns nil if nil.
func (sc *ServiceCheck) Copy() *ServiceCheck {
	if sc == nil {
		return nil
	}
	nsc := new(ServiceCheck)
	*nsc = *sc
	nsc.Args = helper.CopySliceString(sc.Args)
	nsc.Header = helper.CopyMapStringSliceString(sc.Header)
	nsc.CheckRestart = sc.CheckRestart.Copy()
	return nsc
}

// Equals returns true if the structs are recursively equal.
func (sc *ServiceCheck) Equals(o *ServiceCheck) bool {
	if sc == nil || o == nil {
		return sc == o
	}

	if sc.Name != o.Name {
		return false
	}

	if sc.AddressMode != o.AddressMode {
		return false
	}

	if !helper.CompareSliceSetString(sc.Args, o.Args) {
		return false
	}

	if !sc.CheckRestart.Equals(o.CheckRestart) {
		return false
	}

	if sc.TaskName != o.TaskName {
		return false
	}

	if sc.Command != o.Command {
		return false
	}

	if sc.GRPCService != o.GRPCService {
		return false
	}

	if sc.GRPCUseTLS != o.GRPCUseTLS {
		return false
	}

	// Use DeepEqual here as order of slice values could matter
	if !reflect.DeepEqual(sc.Header, o.Header) {
		return false
	}

	if sc.InitialStatus != o.InitialStatus {
		return false
	}

	if sc.Interval != o.Interval {
		return false
	}

	if sc.Method != o.Method {
		return false
	}

	if sc.Path != o.Path {
		return false
	}

	if sc.PortLabel != o.Path {
		return false
	}

	if sc.Protocol != o.Protocol {
		return false
	}

	if sc.TLSSkipVerify != o.TLSSkipVerify {
		return false
	}

	if sc.Timeout != o.Timeout {
		return false
	}

	if sc.Type != o.Type {
		return false
	}

	return true
}

func (sc *ServiceCheck) Canonicalize(serviceName string) {
	// Ensure empty maps/slices are treated as null to avoid scheduling
	// issues when using DeepEquals.
	if len(sc.Args) == 0 {
		sc.Args = nil
	}

	if len(sc.Header) == 0 {
		sc.Header = nil
	} else {
		for k, v := range sc.Header {
			if len(v) == 0 {
				sc.Header[k] = nil
			}
		}
	}

	if sc.Name == "" {
		sc.Name = fmt.Sprintf("service: %q check", serviceName)
	}
}

// validate a Service's ServiceCheck
func (sc *ServiceCheck) validate() error {
	// Validate Type
	switch strings.ToLower(sc.Type) {
	case ServiceCheckGRPC:
	case ServiceCheckTCP:
	case ServiceCheckHTTP:
		if sc.Path == "" {
			return fmt.Errorf("http type must have a valid http path")
		}
		url, err := url.Parse(sc.Path)
		if err != nil {
			return fmt.Errorf("http type must have a valid http path")
		}
		if url.IsAbs() {
			return fmt.Errorf("http type must have a relative http path")
		}

	case ServiceCheckScript:
		if sc.Command == "" {
			return fmt.Errorf("script type must have a valid script path")
		}

	default:
		return fmt.Errorf(`invalid type (%+q), must be one of "http", "tcp", or "script" type`, sc.Type)
	}

	// Validate interval and timeout
	if sc.Interval == 0 {
		return fmt.Errorf("missing required value interval. Interval cannot be less than %v", minCheckInterval)
	} else if sc.Interval < minCheckInterval {
		return fmt.Errorf("interval (%v) cannot be lower than %v", sc.Interval, minCheckInterval)
	}

	if sc.Timeout == 0 {
		return fmt.Errorf("missing required value timeout. Timeout cannot be less than %v", minCheckInterval)
	} else if sc.Timeout < minCheckTimeout {
		return fmt.Errorf("timeout (%v) is lower than required minimum timeout %v", sc.Timeout, minCheckInterval)
	}

	// Validate InitialStatus
	switch sc.InitialStatus {
	case "":
	case api.HealthPassing:
	case api.HealthWarning:
	case api.HealthCritical:
	default:
		return fmt.Errorf(`invalid initial check state (%s), must be one of %q, %q, %q or empty`, sc.InitialStatus, api.HealthPassing, api.HealthWarning, api.HealthCritical)

	}

	// Validate AddressMode
	switch sc.AddressMode {
	case "", AddressModeHost, AddressModeDriver:
		// Ok
	case AddressModeAuto:
		return fmt.Errorf("invalid address_mode %q - %s only valid for services", sc.AddressMode, AddressModeAuto)
	default:
		return fmt.Errorf("invalid address_mode %q", sc.AddressMode)
	}

	return sc.CheckRestart.Validate()
}

// RequiresPort returns whether the service check requires the task has a port.
func (sc *ServiceCheck) RequiresPort() bool {
	switch sc.Type {
	case ServiceCheckGRPC, ServiceCheckHTTP, ServiceCheckTCP:
		return true
	default:
		return false
	}
}

// TriggersRestarts returns true if this check should be watched and trigger a restart
// on failure.
func (sc *ServiceCheck) TriggersRestarts() bool {
	return sc.CheckRestart != nil && sc.CheckRestart.Limit > 0
}

// Hash all ServiceCheck fields and the check's corresponding service ID to
// create an identifier. The identifier is not guaranteed to be unique as if
// the PortLabel is blank, the Service's PortLabel will be used after Hash is
// called.
func (sc *ServiceCheck) Hash(serviceID string) string {
	h := sha1.New()
	io.WriteString(h, serviceID)
	io.WriteString(h, sc.Name)
	io.WriteString(h, sc.Type)
	io.WriteString(h, sc.Command)
	io.WriteString(h, strings.Join(sc.Args, ""))
	io.WriteString(h, sc.Path)
	io.WriteString(h, sc.Protocol)
	io.WriteString(h, sc.PortLabel)
	io.WriteString(h, sc.Interval.String())
	io.WriteString(h, sc.Timeout.String())
	io.WriteString(h, sc.Method)
	// Only include TLSSkipVerify if set to maintain ID stability with Nomad <0.6
	if sc.TLSSkipVerify {
		io.WriteString(h, "true")
	}

	// Since map iteration order isn't stable we need to write k/v pairs to
	// a slice and sort it before hashing.
	if len(sc.Header) > 0 {
		headers := make([]string, 0, len(sc.Header))
		for k, v := range sc.Header {
			headers = append(headers, k+strings.Join(v, ""))
		}
		sort.Strings(headers)
		io.WriteString(h, strings.Join(headers, ""))
	}

	// Only include AddressMode if set to maintain ID stability with Nomad <0.7.1
	if len(sc.AddressMode) > 0 {
		io.WriteString(h, sc.AddressMode)
	}

	// Only include GRPC if set to maintain ID stability with Nomad <0.8.4
	if sc.GRPCService != "" {
		io.WriteString(h, sc.GRPCService)
	}
	if sc.GRPCUseTLS {
		io.WriteString(h, "true")
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

const (
	AddressModeAuto   = "auto"
	AddressModeHost   = "host"
	AddressModeDriver = "driver"
)

// Service represents a Consul service definition
type Service struct {
	// Name of the service registered with Consul. Consul defaults the
	// Name to ServiceID if not specified.  The Name if specified is used
	// as one of the seed values when generating a Consul ServiceID.
	Name string

	// PortLabel is either the numeric port number or the `host:port`.
	// To specify the port number using the host's Consul Advertise
	// address, specify an empty host in the PortLabel (e.g. `:port`).
	PortLabel string

	// AddressMode specifies whether or not to use the host ip:port for
	// this service.
	AddressMode string

	Tags       []string          // List of tags for the service
	CanaryTags []string          // List of tags for the service when it is a canary
	Checks     []*ServiceCheck   // List of checks associated with the service
	Connect    *ConsulConnect    // Consul Connect configuration
	Meta       map[string]string // Consul service meta
}

// Copy the stanza recursively. Returns nil if nil.
func (s *Service) Copy() *Service {
	if s == nil {
		return nil
	}
	ns := new(Service)
	*ns = *s
	ns.Tags = helper.CopySliceString(ns.Tags)
	ns.CanaryTags = helper.CopySliceString(ns.CanaryTags)

	if s.Checks != nil {
		checks := make([]*ServiceCheck, len(ns.Checks))
		for i, c := range ns.Checks {
			checks[i] = c.Copy()
		}
		ns.Checks = checks
	}

	ns.Connect = s.Connect.Copy()

	ns.Meta = helper.CopyMapStringString(s.Meta)

	return ns
}

// Canonicalize interpolates values of Job, Task Group and Task in the Service
// Name. This also generates check names, service id and check ids.
func (s *Service) Canonicalize(job string, taskGroup string, task string) {
	// Ensure empty lists are treated as null to avoid scheduler issues when
	// using DeepEquals
	if len(s.Tags) == 0 {
		s.Tags = nil
	}
	if len(s.CanaryTags) == 0 {
		s.CanaryTags = nil
	}
	if len(s.Checks) == 0 {
		s.Checks = nil
	}

	s.Name = args.ReplaceEnv(s.Name, map[string]string{
		"JOB":       job,
		"TASKGROUP": taskGroup,
		"TASK":      task,
		"BASE":      fmt.Sprintf("%s-%s-%s", job, taskGroup, task),
	},
	)

	for _, check := range s.Checks {
		check.Canonicalize(s.Name)
	}
}

// Validate checks if the Check definition is valid
func (s *Service) Validate() error {
	var mErr multierror.Error

	// Ensure the service name is valid per the below RFCs but make an exception
	// for our interpolation syntax by first stripping any environment variables from the name

	serviceNameStripped := args.ReplaceEnvWithPlaceHolder(s.Name, "ENV-VAR")

	if err := s.ValidateName(serviceNameStripped); err != nil {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Service name must be valid per RFC 1123 and can contain only alphanumeric characters or dashes: %q", s.Name))
	}

	switch s.AddressMode {
	case "", AddressModeAuto, AddressModeHost, AddressModeDriver:
		// OK
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Service address_mode must be %q, %q, or %q; not %q", AddressModeAuto, AddressModeHost, AddressModeDriver, s.AddressMode))
	}

	for _, c := range s.Checks {
		if s.PortLabel == "" && c.PortLabel == "" && c.RequiresPort() {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Check %s invalid: check requires a port but neither check nor service %+q have a port", c.Name, s.Name))
			continue
		}

		// TCP checks against a Consul Connect enabled service are not supported
		// due to the service being bound to the loopback interface inside the
		// network namespace
		if c.Type == ServiceCheckTCP && s.Connect != nil && s.Connect.SidecarService != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Check %s invalid: tcp checks are not valid for Connect enabled services", c.Name))
			continue
		}

		if err := c.validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Check %s invalid: %v", c.Name, err))
		}
	}

	if s.Connect != nil {
		if err := s.Connect.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

// ValidateName checks if the services Name is valid and should be called after
// the name has been interpolated
func (s *Service) ValidateName(name string) error {
	// Ensure the service name is valid per RFC-952 §1
	// (https://tools.ietf.org/html/rfc952), RFC-1123 §2.1
	// (https://tools.ietf.org/html/rfc1123), and RFC-2782
	// (https://tools.ietf.org/html/rfc2782).
	re := regexp.MustCompile(`^(?i:[a-z0-9]|[a-z0-9][a-z0-9\-]{0,61}[a-z0-9])$`)
	if !re.MatchString(name) {
		return fmt.Errorf("Service name must be valid per RFC 1123 and can contain only alphanumeric characters or dashes and must be no longer than 63 characters: %q", name)
	}
	return nil
}

// Hash returns a base32 encoded hash of a Service's contents excluding checks
// as they're hashed independently.
func (s *Service) Hash(allocID, taskName string, canary bool) string {
	h := sha1.New()
	io.WriteString(h, allocID)
	io.WriteString(h, taskName)
	io.WriteString(h, s.Name)
	io.WriteString(h, s.PortLabel)
	io.WriteString(h, s.AddressMode)
	for _, tag := range s.Tags {
		io.WriteString(h, tag)
	}
	for _, tag := range s.CanaryTags {
		io.WriteString(h, tag)
	}
	if len(s.Meta) > 0 {
		fmt.Fprintf(h, "%v", s.Meta)
	}

	// Vary ID on whether or not CanaryTags will be used
	if canary {
		h.Write([]byte("Canary"))
	}

	// Base32 is used for encoding the hash as sha1 hashes can always be
	// encoded without padding, only 4 bytes larger than base64, and saves
	// 8 bytes vs hex. Since these hashes are used in Consul URLs it's nice
	// to have a reasonably compact URL-safe representation.
	return b32.EncodeToString(h.Sum(nil))
}

// Equals returns true if the structs are recursively equal.
func (s *Service) Equals(o *Service) bool {
	if s == nil || o == nil {
		return s == o
	}

	if s.AddressMode != o.AddressMode {
		return false
	}

	if !helper.CompareSliceSetString(s.CanaryTags, o.CanaryTags) {
		return false
	}

	if len(s.Checks) != len(o.Checks) {
		return false
	}

OUTER:
	for i := range s.Checks {
		for ii := range o.Checks {
			if s.Checks[i].Equals(o.Checks[ii]) {
				// Found match; continue with next check
				continue OUTER
			}
		}

		// No match
		return false
	}

	if !s.Connect.Equals(o.Connect) {
		return false
	}

	if s.Name != o.Name {
		return false
	}

	if s.PortLabel != o.PortLabel {
		return false
	}

	if !reflect.DeepEqual(s.Meta, o.Meta) {
		return false
	}

	if !helper.CompareSliceSetString(s.Tags, o.Tags) {
		return false
	}

	return true
}

// ConsulConnect represents a Consul Connect jobspec stanza.
type ConsulConnect struct {
	// Native is true if a service implements Connect directly and does not
	// need a sidecar.
	Native bool

	// SidecarService is non-nil if a service requires a sidecar.
	SidecarService *ConsulSidecarService

	// SidecarTask is non-nil if sidecar overrides are set
	SidecarTask *SidecarTask
}

// Copy the stanza recursively. Returns nil if nil.
func (c *ConsulConnect) Copy() *ConsulConnect {
	if c == nil {
		return nil
	}

	return &ConsulConnect{
		Native:         c.Native,
		SidecarService: c.SidecarService.Copy(),
		SidecarTask:    c.SidecarTask.Copy(),
	}
}

// Equals returns true if the structs are recursively equal.
func (c *ConsulConnect) Equals(o *ConsulConnect) bool {
	if c == nil || o == nil {
		return c == o
	}

	if c.Native != o.Native {
		return false
	}

	return c.SidecarService.Equals(o.SidecarService)
}

// HasSidecar checks if a sidecar task is needed
func (c *ConsulConnect) HasSidecar() bool {
	return c != nil && c.SidecarService != nil
}

// Validate that the Connect stanza has exactly one of Native or sidecar.
func (c *ConsulConnect) Validate() error {
	if c == nil {
		return nil
	}

	if c.Native && c.SidecarService != nil {
		return fmt.Errorf("Consul Connect must be native or use a sidecar service; not both")
	}

	if !c.Native && c.SidecarService == nil {
		return fmt.Errorf("Consul Connect must be native or use a sidecar service")
	}

	return nil
}

// ConsulSidecarService represents a Consul Connect SidecarService jobspec
// stanza.
type ConsulSidecarService struct {
	// Tags are optional service tags that get registered with the sidecar service
	// in Consul. If unset, the sidecar service inherits the parent service tags.
	Tags []string

	// Port is the service's port that the sidecar will connect to. May be
	// a port label or a literal port number.
	Port string

	// Proxy stanza defining the sidecar proxy configuration.
	Proxy *ConsulProxy
}

// HasUpstreams checks if the sidecar service has any upstreams configured
func (s *ConsulSidecarService) HasUpstreams() bool {
	return s != nil && s.Proxy != nil && len(s.Proxy.Upstreams) > 0
}

// Copy the stanza recursively. Returns nil if nil.
func (s *ConsulSidecarService) Copy() *ConsulSidecarService {
	return &ConsulSidecarService{
		Tags:  helper.CopySliceString(s.Tags),
		Port:  s.Port,
		Proxy: s.Proxy.Copy(),
	}
}

// Equals returns true if the structs are recursively equal.
func (s *ConsulSidecarService) Equals(o *ConsulSidecarService) bool {
	if s == nil || o == nil {
		return s == o
	}

	if s.Port != o.Port {
		return false
	}

	if !helper.CompareSliceSetString(s.Tags, o.Tags) {
		return false
	}

	return s.Proxy.Equals(o.Proxy)
}

// SidecarTask represents a subset of Task fields that are able to be overridden
// from the sidecar_task stanza
type SidecarTask struct {
	// Name of the task
	Name string

	// Driver is used to control which driver is used
	Driver string

	// User is used to determine which user will run the task. It defaults to
	// the same user the Nomad client is being run as.
	User string

	// Config is provided to the driver to initialize
	Config map[string]interface{}

	// Map of environment variables to be used by the driver
	Env map[string]string

	// Resources is the resources needed by this task
	Resources *Resources

	// Meta is used to associate arbitrary metadata with this
	// task. This is opaque to Nomad.
	Meta map[string]string

	// KillTimeout is the time between signaling a task that it will be
	// killed and killing it.
	KillTimeout *time.Duration

	// LogConfig provides configuration for log rotation
	LogConfig *LogConfig

	// ShutdownDelay is the duration of the delay between deregistering a
	// task from Consul and sending it a signal to shutdown. See #2441
	ShutdownDelay *time.Duration

	// KillSignal is the kill signal to use for the task. This is an optional
	// specification and defaults to SIGINT
	KillSignal string
}

func (t *SidecarTask) Copy() *SidecarTask {
	if t == nil {
		return nil
	}
	nt := new(SidecarTask)
	*nt = *t
	nt.Env = helper.CopyMapStringString(nt.Env)

	nt.Resources = nt.Resources.Copy()
	nt.LogConfig = nt.LogConfig.Copy()
	nt.Meta = helper.CopyMapStringString(nt.Meta)

	if i, err := copystructure.Copy(nt.Config); err != nil {
		panic(err.Error())
	} else {
		nt.Config = i.(map[string]interface{})
	}

	if t.KillTimeout != nil {
		nt.KillTimeout = helper.TimeToPtr(*t.KillTimeout)
	}

	if t.ShutdownDelay != nil {
		nt.ShutdownDelay = helper.TimeToPtr(*t.ShutdownDelay)
	}

	return nt
}

// MergeIntoTask merges the SidecarTask fields over the given task
func (t *SidecarTask) MergeIntoTask(task *Task) {
	if t.Name != "" {
		task.Name = t.Name
	}

	// If the driver changes then the driver config can be overwritten.
	// Otherwise we'll merge the driver config together
	if t.Driver != "" && t.Driver != task.Driver {
		task.Driver = t.Driver
		task.Config = t.Config
	} else {
		for k, v := range t.Config {
			task.Config[k] = v
		}
	}

	if t.User != "" {
		task.User = t.User
	}

	if t.Env != nil {
		if task.Env == nil {
			task.Env = t.Env
		} else {
			for k, v := range t.Env {
				task.Env[k] = v
			}
		}
	}

	if t.Resources != nil {
		task.Resources.Merge(t.Resources)
	}

	if t.Meta != nil {
		if task.Meta == nil {
			task.Meta = t.Meta
		} else {
			for k, v := range t.Meta {
				task.Meta[k] = v
			}
		}
	}

	if t.KillTimeout != nil {
		task.KillTimeout = *t.KillTimeout
	}

	if t.LogConfig != nil {
		if task.LogConfig == nil {
			task.LogConfig = t.LogConfig
		} else {
			if t.LogConfig.MaxFiles > 0 {
				task.LogConfig.MaxFiles = t.LogConfig.MaxFiles
			}
			if t.LogConfig.MaxFileSizeMB > 0 {
				task.LogConfig.MaxFileSizeMB = t.LogConfig.MaxFileSizeMB
			}
		}
	}

	if t.ShutdownDelay != nil {
		task.ShutdownDelay = *t.ShutdownDelay
	}

	if t.KillSignal != "" {
		task.KillSignal = t.KillSignal
	}
}

// ConsulProxy represents a Consul Connect sidecar proxy jobspec stanza.
type ConsulProxy struct {

	// LocalServiceAddress is the address the local service binds to.
	// Usually 127.0.0.1 it is useful to customize in clusters with mixed
	// Connect and non-Connect services.
	LocalServiceAddress string

	// LocalServicePort is the port the local service binds to. Usually
	// the same as the parent service's port, it is useful to customize
	// in clusters with mixed Connect and non-Connect services
	LocalServicePort int

	// Upstreams configures the upstream services this service intends to
	// connect to.
	Upstreams []ConsulUpstream

	// Config is a proxy configuration. It is opaque to Nomad and passed
	// directly to Consul.
	Config map[string]interface{}
}

// Copy the stanza recursively. Returns nil if nil.
func (p *ConsulProxy) Copy() *ConsulProxy {
	if p == nil {
		return nil
	}

	newP := ConsulProxy{}
	newP.LocalServiceAddress = p.LocalServiceAddress
	newP.LocalServicePort = p.LocalServicePort

	if n := len(p.Upstreams); n > 0 {
		newP.Upstreams = make([]ConsulUpstream, n)

		for i := range p.Upstreams {
			newP.Upstreams[i] = *p.Upstreams[i].Copy()
		}
	}

	if n := len(p.Config); n > 0 {
		newP.Config = make(map[string]interface{}, n)

		for k, v := range p.Config {
			newP.Config[k] = v
		}
	}

	return &newP
}

// Equals returns true if the structs are recursively equal.
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
			if up.Equals(&innerUp) {
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

// ConsulUpstream represents a Consul Connect upstream jobspec stanza.
type ConsulUpstream struct {
	// DestinationName is the name of the upstream service.
	DestinationName string

	// LocalBindPort is the port the proxy will receive connections for the
	// upstream on.
	LocalBindPort int
}

// Copy the stanza recursively. Returns nil if nil.
func (u *ConsulUpstream) Copy() *ConsulUpstream {
	if u == nil {
		return nil
	}

	return &ConsulUpstream{
		DestinationName: u.DestinationName,
		LocalBindPort:   u.LocalBindPort,
	}
}

// Equals returns true if the structs are recursively equal.
func (u *ConsulUpstream) Equals(o *ConsulUpstream) bool {
	if u == nil || o == nil {
		return u == o
	}

	return (*u) == (*o)
}
