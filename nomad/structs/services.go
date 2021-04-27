package structs

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
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
	Name                   string              // Name of the check, defaults to id
	Type                   string              // Type of the check - tcp, http, docker and script
	Command                string              // Command is the command to run for script checks
	Args                   []string            // Args is a list of arguments for script checks
	Path                   string              // path of the health check url for http type check
	Protocol               string              // Protocol to use if check is http, defaults to http
	PortLabel              string              // The port to use for tcp/http checks
	Expose                 bool                // Whether to have Envoy expose the check path (connect-enabled group-services only)
	AddressMode            string              // 'host' to use host ip:port or 'driver' to use driver's
	Interval               time.Duration       // Interval of the check
	Timeout                time.Duration       // Timeout of the response from the check before consul fails the check
	InitialStatus          string              // Initial status of the check
	TLSSkipVerify          bool                // Skip TLS verification when Protocol=https
	Method                 string              // HTTP Method to use (GET by default)
	Header                 map[string][]string // HTTP Headers for Consul to set when making HTTP checks
	CheckRestart           *CheckRestart       // If and when a task should be restarted based on checks
	GRPCService            string              // Service for GRPC checks
	GRPCUseTLS             bool                // Whether or not to use TLS for GRPC checks
	TaskName               string              // What task to execute this check in
	SuccessBeforePassing   int                 // Number of consecutive successes required before considered healthy
	FailuresBeforeCritical int                 // Number of consecutive failures required before considered unhealthy
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

	if sc.SuccessBeforePassing != o.SuccessBeforePassing {
		return false
	}

	if sc.FailuresBeforeCritical != o.FailuresBeforeCritical {
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

	if sc.Expose != o.Expose {
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
	checkType := strings.ToLower(sc.Type)
	switch checkType {
	case ServiceCheckGRPC:
	case ServiceCheckTCP:
	case ServiceCheckHTTP:
		if sc.Path == "" {
			return fmt.Errorf("http type must have a valid http path")
		}
		checkPath, err := url.Parse(sc.Path)
		if err != nil {
			return fmt.Errorf("http type must have a valid http path")
		}
		if checkPath.IsAbs() {
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
	case "", AddressModeHost, AddressModeDriver, AddressModeAlloc:
		// Ok
	case AddressModeAuto:
		return fmt.Errorf("invalid address_mode %q - %s only valid for services", sc.AddressMode, AddressModeAuto)
	default:
		return fmt.Errorf("invalid address_mode %q", sc.AddressMode)
	}

	// Note that we cannot completely validate the Expose field yet - we do not
	// know whether this ServiceCheck belongs to a connect-enabled group-service.
	// Instead, such validation will happen in a job admission controller.
	if sc.Expose {
		// We can however immediately ensure expose is configured only for HTTP
		// and gRPC checks.
		switch checkType {
		case ServiceCheckGRPC, ServiceCheckHTTP: // ok
		default:
			return fmt.Errorf("expose may only be set on HTTP or gRPC checks")
		}
	}

	// passFailCheckTypes are intersection of check types supported by both Consul
	// and Nomad when using the pass/fail check threshold features.
	passFailCheckTypes := []string{"tcp", "http", "grpc"}

	if sc.SuccessBeforePassing < 0 {
		return fmt.Errorf("success_before_passing must be non-negative")
	} else if sc.SuccessBeforePassing > 0 && !helper.SliceStringContains(passFailCheckTypes, sc.Type) {
		return fmt.Errorf("success_before_passing not supported for check of type %q", sc.Type)
	}

	if sc.FailuresBeforeCritical < 0 {
		return fmt.Errorf("failures_before_critical must be non-negative")
	} else if sc.FailuresBeforeCritical > 0 && !helper.SliceStringContains(passFailCheckTypes, sc.Type) {
		return fmt.Errorf("failures_before_critical not supported for check of type %q", sc.Type)
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
	hashString(h, serviceID)
	hashString(h, sc.Name)
	hashString(h, sc.Type)
	hashString(h, sc.Command)
	hashString(h, strings.Join(sc.Args, ""))
	hashString(h, sc.Path)
	hashString(h, sc.Protocol)
	hashString(h, sc.PortLabel)
	hashString(h, sc.Interval.String())
	hashString(h, sc.Timeout.String())
	hashString(h, sc.Method)

	// use name "true" to maintain ID stability
	hashBool(h, sc.TLSSkipVerify, "true")

	// maintain artisanal map hashing to maintain ID stability
	hashHeader(h, sc.Header)

	// Only include AddressMode if set to maintain ID stability with Nomad <0.7.1
	hashStringIfNonEmpty(h, sc.AddressMode)

	// Only include gRPC if set to maintain ID stability with Nomad <0.8.4
	hashStringIfNonEmpty(h, sc.GRPCService)

	// use name "true" to maintain ID stability
	hashBool(h, sc.GRPCUseTLS, "true")

	// Only include pass/fail if non-zero to maintain ID stability with Nomad < 0.12
	hashIntIfNonZero(h, "success", sc.SuccessBeforePassing)
	hashIntIfNonZero(h, "failures", sc.FailuresBeforeCritical)

	// Hash is used for diffing against the Consul check definition, which does
	// not have an expose parameter. Instead we rely on implied changes to
	// other fields if the Expose setting is changed in a nomad service.
	// hashBool(h, sc.Expose, "Expose")

	// maintain use of hex (i.e. not b32) to maintain ID stability
	return fmt.Sprintf("%x", h.Sum(nil))
}

func hashStringIfNonEmpty(h hash.Hash, s string) {
	if len(s) > 0 {
		hashString(h, s)
	}
}

func hashIntIfNonZero(h hash.Hash, name string, i int) {
	if i != 0 {
		hashString(h, fmt.Sprintf("%s:%d", name, i))
	}
}

func hashHeader(h hash.Hash, m map[string][]string) {
	// maintain backwards compatibility for ID stability
	// using the %v formatter on a map with string keys produces consistent
	// output, but our existing format here is incompatible
	if len(m) > 0 {
		headers := make([]string, 0, len(m))
		for k, v := range m {
			headers = append(headers, k+strings.Join(v, ""))
		}
		sort.Strings(headers)
		hashString(h, strings.Join(headers, ""))
	}
}

const (
	AddressModeAuto   = "auto"
	AddressModeHost   = "host"
	AddressModeDriver = "driver"
	AddressModeAlloc  = "alloc"
)

// Service represents a Consul service definition
type Service struct {
	// Name of the service registered with Consul. Consul defaults the
	// Name to ServiceID if not specified.  The Name if specified is used
	// as one of the seed values when generating a Consul ServiceID.
	Name string

	// Name of the Task associated with this service.
	//
	// Currently only used to identify the implementing task of a Consul
	// Connect Native enabled service.
	TaskName string

	// PortLabel is either the numeric port number or the `host:port`.
	// To specify the port number using the host's Consul Advertise
	// address, specify an empty host in the PortLabel (e.g. `:port`).
	PortLabel string

	// AddressMode specifies whether or not to use the host ip:port for
	// this service.
	AddressMode string

	// EnableTagOverride will disable Consul's anti-entropy mechanism for the
	// tags of this service. External updates to the service definition via
	// Consul will not be corrected to match the service definition set in the
	// Nomad job specification.
	//
	// https://www.consul.io/docs/agent/services.html#service-definition
	EnableTagOverride bool

	Tags       []string          // List of tags for the service
	CanaryTags []string          // List of tags for the service when it is a canary
	Checks     []*ServiceCheck   // List of checks associated with the service
	Connect    *ConsulConnect    // Consul Connect configuration
	Meta       map[string]string // Consul service meta
	CanaryMeta map[string]string // Consul service meta when it is a canary
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
	ns.CanaryMeta = helper.CopyMapStringString(s.CanaryMeta)

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
	})

	for _, check := range s.Checks {
		check.Canonicalize(s.Name)
	}
}

// Validate checks if the Service definition is valid
func (s *Service) Validate() error {
	var mErr multierror.Error

	// Ensure the service name is valid per the below RFCs but make an exception
	// for our interpolation syntax by first stripping any environment variables from the name

	serviceNameStripped := args.ReplaceEnvWithPlaceHolder(s.Name, "ENV-VAR")

	if err := s.ValidateName(serviceNameStripped); err != nil {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Service name must be valid per RFC 1123 and can contain only alphanumeric characters or dashes: %q", s.Name))
	}

	switch s.AddressMode {
	case "", AddressModeAuto, AddressModeHost, AddressModeDriver, AddressModeAlloc:
		// OK
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Service address_mode must be %q, %q, or %q; not %q", AddressModeAuto, AddressModeHost, AddressModeDriver, s.AddressMode))
	}

	// check checks
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

	// check connect
	if s.Connect != nil {
		if err := s.Connect.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}

		// if service is connect native, service task must be set (which may
		// happen implicitly in a job mutation if there is only one task)
		if s.Connect.IsNative() && len(s.TaskName) == 0 {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Service %s is Connect Native and requires setting the task", s.Name))
		}
	}

	return mErr.ErrorOrNil()
}

// ValidateName checks if the service Name is valid and should be called after
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
	hashString(h, allocID)
	hashString(h, taskName)
	hashString(h, s.Name)
	hashString(h, s.PortLabel)
	hashString(h, s.AddressMode)
	hashTags(h, s.Tags)
	hashTags(h, s.CanaryTags)
	hashBool(h, canary, "Canary")
	hashBool(h, s.EnableTagOverride, "ETO")
	hashMeta(h, s.Meta)
	hashMeta(h, s.CanaryMeta)
	hashConnect(h, s.Connect)

	// Base32 is used for encoding the hash as sha1 hashes can always be
	// encoded without padding, only 4 bytes larger than base64, and saves
	// 8 bytes vs hex. Since these hashes are used in Consul URLs it's nice
	// to have a reasonably compact URL-safe representation.
	return b32.EncodeToString(h.Sum(nil))
}

func hashConnect(h hash.Hash, connect *ConsulConnect) {
	if connect != nil && connect.SidecarService != nil {
		hashString(h, connect.SidecarService.Port)
		hashTags(h, connect.SidecarService.Tags)
		if p := connect.SidecarService.Proxy; p != nil {
			hashString(h, p.LocalServiceAddress)
			hashString(h, strconv.Itoa(p.LocalServicePort))
			hashConfig(h, p.Config)
			for _, upstream := range p.Upstreams {
				hashString(h, upstream.DestinationName)
				hashString(h, strconv.Itoa(upstream.LocalBindPort))
				hashStringIfNonEmpty(h, upstream.Datacenter)
			}
		}
	}
}

func hashString(h hash.Hash, s string) {
	_, _ = io.WriteString(h, s)
}

func hashBool(h hash.Hash, b bool, name string) {
	if b {
		hashString(h, name)
	}
}

func hashTags(h hash.Hash, tags []string) {
	for _, tag := range tags {
		hashString(h, tag)
	}
}

func hashMeta(h hash.Hash, m map[string]string) {
	_, _ = fmt.Fprintf(h, "%v", m)
}

func hashConfig(h hash.Hash, c map[string]interface{}) {
	_, _ = fmt.Fprintf(h, "%v", c)
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

	if !reflect.DeepEqual(s.CanaryMeta, o.CanaryMeta) {
		return false
	}

	if !helper.CompareSliceSetString(s.Tags, o.Tags) {
		return false
	}

	if s.EnableTagOverride != o.EnableTagOverride {
		return false
	}

	return true
}

// ConsulConnect represents a Consul Connect jobspec stanza.
type ConsulConnect struct {
	// Native indicates whether the service is Consul Connect Native enabled.
	Native bool

	// SidecarService is non-nil if a service requires a sidecar.
	SidecarService *ConsulSidecarService

	// SidecarTask is non-nil if sidecar overrides are set
	SidecarTask *SidecarTask

	// Gateway is a Consul Connect Gateway Proxy.
	Gateway *ConsulGateway
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
		Gateway:        c.Gateway.Copy(),
	}
}

// Equals returns true if the connect blocks are deeply equal.
func (c *ConsulConnect) Equals(o *ConsulConnect) bool {
	if c == nil || o == nil {
		return c == o
	}

	if c.Native != o.Native {
		return false
	}

	if !c.SidecarService.Equals(o.SidecarService) {
		return false
	}

	if !c.SidecarTask.Equals(o.SidecarTask) {
		return false
	}

	if !c.Gateway.Equals(o.Gateway) {
		return false
	}

	return true
}

// HasSidecar checks if a sidecar task is configured.
func (c *ConsulConnect) HasSidecar() bool {
	return c != nil && c.SidecarService != nil
}

// IsNative checks if the service is connect native.
func (c *ConsulConnect) IsNative() bool {
	return c != nil && c.Native
}

// IsGateway checks if the service is any type of connect gateway.
func (c *ConsulConnect) IsGateway() bool {
	return c != nil && c.Gateway != nil
}

// IsIngress checks if the service is an ingress gateway.
func (c *ConsulConnect) IsIngress() bool {
	return c.IsGateway() && c.Gateway.Ingress != nil
}

// IsTerminating checks if the service is a terminating gateway.
func (c *ConsulConnect) IsTerminating() bool {
	return c.IsGateway() && c.Gateway.Terminating != nil
}

// also mesh

// Validate that the Connect block represents exactly one of:
// - Connect non-native service sidecar proxy
// - Connect native service
// - Connect gateway (any type)
func (c *ConsulConnect) Validate() error {
	if c == nil {
		return nil
	}

	// Count the number of things actually configured. If that number is not 1,
	// the config is not valid.
	count := 0

	if c.HasSidecar() {
		count++
	}

	if c.IsNative() {
		count++
	}

	if c.IsGateway() {
		count++
	}

	if count != 1 {
		return fmt.Errorf("Consul Connect must be exclusively native, make use of a sidecar, or represent a Gateway")
	}

	if c.IsGateway() {
		if err := c.Gateway.Validate(); err != nil {
			return err
		}
	}

	// The Native and Sidecar cases are validated up at the service level.

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
	if s == nil {
		return nil
	}
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

func (t *SidecarTask) Equals(o *SidecarTask) bool {
	if t == nil || o == nil {
		return t == o
	}

	if t.Name != o.Name {
		return false
	}

	if t.Driver != o.Driver {
		return false
	}

	if t.User != o.User {
		return false
	}

	// config compare
	if !opaqueMapsEqual(t.Config, o.Config) {
		return false
	}

	if !helper.CompareMapStringString(t.Env, o.Env) {
		return false
	}

	if !t.Resources.Equals(o.Resources) {
		return false
	}

	if !helper.CompareMapStringString(t.Meta, o.Meta) {
		return false
	}

	if !helper.CompareTimePtrs(t.KillTimeout, o.KillTimeout) {
		return false
	}

	if !t.LogConfig.Equals(o.LogConfig) {
		return false
	}

	if !helper.CompareTimePtrs(t.ShutdownDelay, o.ShutdownDelay) {
		return false
	}

	if t.KillSignal != o.KillSignal {
		return false
	}

	return true
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

	// Expose configures the consul proxy.expose stanza to "open up" endpoints
	// used by task-group level service checks using HTTP or gRPC protocols.
	//
	// Use json tag to match with field name in api/
	Expose *ConsulExposeConfig `json:"ExposeConfig"`

	// Config is a proxy configuration. It is opaque to Nomad and passed
	// directly to Consul.
	Config map[string]interface{}
}

// Copy the stanza recursively. Returns nil if nil.
func (p *ConsulProxy) Copy() *ConsulProxy {
	if p == nil {
		return nil
	}

	newP := &ConsulProxy{
		LocalServiceAddress: p.LocalServiceAddress,
		LocalServicePort:    p.LocalServicePort,
		Expose:              p.Expose.Copy(),
	}

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

	return newP
}

// opaqueMapsEqual compares map[string]interface{} commonly used for opaque
// config blocks. Interprets nil and {} as the same.
func opaqueMapsEqual(a, b map[string]interface{}) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
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

	if !p.Expose.Equals(o.Expose) {
		return false
	}

	if !upstreamsEquals(p.Upstreams, o.Upstreams) {
		return false
	}

	if !opaqueMapsEqual(p.Config, o.Config) {
		return false
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

	// Datacenter is the datacenter in which to issue the discovery query to.
	Datacenter string
}

func upstreamsEquals(a, b []ConsulUpstream) bool {
	if len(a) != len(b) {
		return false
	}

LOOP: // order does not matter
	for _, upA := range a {
		for _, upB := range b {
			if upA.Equals(&upB) {
				continue LOOP
			}
		}
		return false
	}
	return true
}

// Copy the stanza recursively. Returns nil if u is nil.
func (u *ConsulUpstream) Copy() *ConsulUpstream {
	if u == nil {
		return nil
	}

	return &ConsulUpstream{
		DestinationName: u.DestinationName,
		LocalBindPort:   u.LocalBindPort,
		Datacenter:      u.Datacenter,
	}
}

// Equals returns true if the structs are recursively equal.
func (u *ConsulUpstream) Equals(o *ConsulUpstream) bool {
	if u == nil || o == nil {
		return u == o
	}

	return (*u) == (*o)
}

// ExposeConfig represents a Consul Connect expose jobspec stanza.
type ConsulExposeConfig struct {
	// Use json tag to match with field name in api/
	Paths []ConsulExposePath `json:"Path"`
}

type ConsulExposePath struct {
	Path          string
	Protocol      string
	LocalPathPort int
	ListenerPort  string
}

func exposePathsEqual(pathsA, pathsB []ConsulExposePath) bool {
	if len(pathsA) != len(pathsB) {
		return false
	}

LOOP: // order does not matter
	for _, pathA := range pathsA {
		for _, pathB := range pathsB {
			if pathA == pathB {
				continue LOOP
			}
		}
		return false
	}
	return true
}

// Copy the stanza. Returns nil if e is nil.
func (e *ConsulExposeConfig) Copy() *ConsulExposeConfig {
	if e == nil {
		return nil
	}
	paths := make([]ConsulExposePath, len(e.Paths))
	for i := 0; i < len(e.Paths); i++ {
		paths[i] = e.Paths[i]
	}
	return &ConsulExposeConfig{
		Paths: paths,
	}
}

// Equals returns true if the structs are recursively equal.
func (e *ConsulExposeConfig) Equals(o *ConsulExposeConfig) bool {
	if e == nil || o == nil {
		return e == o
	}
	return exposePathsEqual(e.Paths, o.Paths)
}

// ConsulGateway is used to configure one of the Consul Connect Gateway types.
type ConsulGateway struct {
	// Proxy is used to configure the Envoy instance acting as the gateway.
	Proxy *ConsulGatewayProxy

	// Ingress represents the Consul Configuration Entry for an Ingress Gateway.
	Ingress *ConsulIngressConfigEntry

	// Terminating represents the Consul Configuration Entry for a Terminating Gateway.
	Terminating *ConsulTerminatingConfigEntry

	// Mesh is not yet supported.
	// Mesh *ConsulMeshConfigEntry
}

func (g *ConsulGateway) Prefix() string {
	switch {
	case g.Ingress != nil:
		return ConnectIngressPrefix
	default:
		return ConnectTerminatingPrefix
	}
	// also mesh
}

func (g *ConsulGateway) Copy() *ConsulGateway {
	if g == nil {
		return nil
	}

	return &ConsulGateway{
		Proxy:       g.Proxy.Copy(),
		Ingress:     g.Ingress.Copy(),
		Terminating: g.Terminating.Copy(),
	}
}

func (g *ConsulGateway) Equals(o *ConsulGateway) bool {
	if g == nil || o == nil {
		return g == o
	}

	if !g.Proxy.Equals(o.Proxy) {
		return false
	}

	if !g.Ingress.Equals(o.Ingress) {
		return false
	}

	if !g.Terminating.Equals(o.Terminating) {
		return false
	}

	return true
}

func (g *ConsulGateway) Validate() error {
	if g == nil {
		return nil
	}

	if err := g.Proxy.Validate(); err != nil {
		return err
	}

	if err := g.Ingress.Validate(); err != nil {
		return err
	}

	if err := g.Terminating.Validate(); err != nil {
		return err
	}

	// Exactly 1 of ingress/terminating/mesh(soon) must be set.
	count := 0
	if g.Ingress != nil {
		count++
	}
	if g.Terminating != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("One Consul Gateway Configuration Entry must be set")
	}
	return nil
}

// ConsulGatewayBindAddress is equivalent to Consul's api/catalog.go ServiceAddress
// struct, as this is used to encode values to pass along to Envoy (i.e. via
// JSON encoding).
type ConsulGatewayBindAddress struct {
	Address string
	Port    int
}

func (a *ConsulGatewayBindAddress) Equals(o *ConsulGatewayBindAddress) bool {
	if a == nil || o == nil {
		return a == o
	}

	if a.Address != o.Address {
		return false
	}

	if a.Port != o.Port {
		return false
	}

	return true
}

func (a *ConsulGatewayBindAddress) Copy() *ConsulGatewayBindAddress {
	if a == nil {
		return nil
	}

	return &ConsulGatewayBindAddress{
		Address: a.Address,
		Port:    a.Port,
	}
}

func (a *ConsulGatewayBindAddress) Validate() error {
	if a == nil {
		return nil
	}

	if a.Address == "" {
		return fmt.Errorf("Consul Gateway Bind Address must be set")
	}

	if a.Port <= 0 && a.Port != -1 { // port -1 => nomad autofill
		return fmt.Errorf("Consul Gateway Bind Address must set valid Port")
	}

	return nil
}

// ConsulGatewayProxy is used to tune parameters of the proxy instance acting as
// one of the forms of Connect gateways that Consul supports.
//
// https://www.consul.io/docs/connect/proxies/envoy#gateway-options
type ConsulGatewayProxy struct {
	ConnectTimeout                  *time.Duration
	EnvoyGatewayBindTaggedAddresses bool
	EnvoyGatewayBindAddresses       map[string]*ConsulGatewayBindAddress
	EnvoyGatewayNoDefaultBind       bool
	EnvoyDNSDiscoveryType           string
	Config                          map[string]interface{}
}

func (p *ConsulGatewayProxy) Copy() *ConsulGatewayProxy {
	if p == nil {
		return nil
	}

	return &ConsulGatewayProxy{
		ConnectTimeout:                  helper.TimeToPtr(*p.ConnectTimeout),
		EnvoyGatewayBindTaggedAddresses: p.EnvoyGatewayBindTaggedAddresses,
		EnvoyGatewayBindAddresses:       p.copyBindAddresses(),
		EnvoyGatewayNoDefaultBind:       p.EnvoyGatewayNoDefaultBind,
		EnvoyDNSDiscoveryType:           p.EnvoyDNSDiscoveryType,
		Config:                          helper.CopyMapStringInterface(p.Config),
	}
}

func (p *ConsulGatewayProxy) copyBindAddresses() map[string]*ConsulGatewayBindAddress {
	if p.EnvoyGatewayBindAddresses == nil {
		return nil
	}

	bindAddresses := make(map[string]*ConsulGatewayBindAddress, len(p.EnvoyGatewayBindAddresses))
	for k, v := range p.EnvoyGatewayBindAddresses {
		bindAddresses[k] = v.Copy()
	}

	return bindAddresses
}

func (p *ConsulGatewayProxy) equalBindAddresses(o map[string]*ConsulGatewayBindAddress) bool {
	if len(p.EnvoyGatewayBindAddresses) != len(o) {
		return false
	}

	for listener, addr := range p.EnvoyGatewayBindAddresses {
		if !o[listener].Equals(addr) {
			return false
		}
	}

	return true
}

func (p *ConsulGatewayProxy) Equals(o *ConsulGatewayProxy) bool {
	if p == nil || o == nil {
		return p == o
	}

	if !helper.CompareTimePtrs(p.ConnectTimeout, o.ConnectTimeout) {
		return false
	}

	if p.EnvoyGatewayBindTaggedAddresses != o.EnvoyGatewayBindTaggedAddresses {
		return false
	}

	if !p.equalBindAddresses(o.EnvoyGatewayBindAddresses) {
		return false
	}

	if p.EnvoyGatewayNoDefaultBind != o.EnvoyGatewayNoDefaultBind {
		return false
	}

	if p.EnvoyDNSDiscoveryType != o.EnvoyDNSDiscoveryType {
		return false
	}

	if !opaqueMapsEqual(p.Config, o.Config) {
		return false
	}

	return true
}

const (
	strictDNS  = "STRICT_DNS"
	logicalDNS = "LOGICAL_DNS"
)

func (p *ConsulGatewayProxy) Validate() error {
	if p == nil {
		return nil
	}

	if p.ConnectTimeout == nil {
		return fmt.Errorf("Consul Gateway Proxy connection_timeout must be set")
	}

	switch p.EnvoyDNSDiscoveryType {
	case "", strictDNS, logicalDNS:
		// Consul defaults to logical DNS, suitable for large scale workloads.
		// https://www.envoyproxy.io/docs/envoy/v1.16.1/intro/arch_overview/upstream/service_discovery
	default:
		return fmt.Errorf("Consul Gateway Proxy Envoy DNS Discovery type must be %s or %s", strictDNS, logicalDNS)
	}

	for _, bindAddr := range p.EnvoyGatewayBindAddresses {
		if err := bindAddr.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ConsulGatewayTLSConfig is used to configure TLS for a gateway.
type ConsulGatewayTLSConfig struct {
	Enabled bool
}

func (c *ConsulGatewayTLSConfig) Copy() *ConsulGatewayTLSConfig {
	if c == nil {
		return nil
	}

	return &ConsulGatewayTLSConfig{
		Enabled: c.Enabled,
	}
}

func (c *ConsulGatewayTLSConfig) Equals(o *ConsulGatewayTLSConfig) bool {
	if c == nil || o == nil {
		return c == o
	}

	return c.Enabled == o.Enabled
}

// ConsulIngressService is used to configure a service fronted by the ingress gateway.
type ConsulIngressService struct {
	// Namespace is not yet supported.
	// Namespace string

	Name string

	Hosts []string
}

func (s *ConsulIngressService) Copy() *ConsulIngressService {
	if s == nil {
		return nil
	}

	var hosts []string = nil
	if n := len(s.Hosts); n > 0 {
		hosts = make([]string, n)
		copy(hosts, s.Hosts)
	}

	return &ConsulIngressService{
		Name:  s.Name,
		Hosts: hosts,
	}
}

func (s *ConsulIngressService) Equals(o *ConsulIngressService) bool {
	if s == nil || o == nil {
		return s == o
	}

	if s.Name != o.Name {
		return false
	}

	return helper.CompareSliceSetString(s.Hosts, o.Hosts)
}

func (s *ConsulIngressService) Validate(isHTTP bool) error {
	if s == nil {
		return nil
	}

	if s.Name == "" {
		return errors.New("Consul Ingress Service requires a name")
	}

	// Validation of wildcard service name and hosts varies on whether the protocol
	// for the gateway is HTTP.
	// https://www.consul.io/docs/connect/config-entries/ingress-gateway#hosts
	switch isHTTP {
	case true:
		if s.Name == "*" {
			return nil
		}

		if len(s.Hosts) == 0 {
			return errors.New("Consul Ingress Service requires one or more hosts when using HTTP protocol")
		}
	case false:
		if s.Name == "*" {
			return errors.New("Consul Ingress Service supports wildcard names only with HTTP protocol")
		}

		if len(s.Hosts) > 0 {
			return errors.New("Consul Ingress Service supports hosts only when using HTTP protocol")
		}
	}

	return nil
}

// ConsulIngressListener is used to configure a listener on a Consul Ingress
// Gateway.
type ConsulIngressListener struct {
	Port     int
	Protocol string
	Services []*ConsulIngressService
}

func (l *ConsulIngressListener) Copy() *ConsulIngressListener {
	if l == nil {
		return nil
	}

	var services []*ConsulIngressService = nil
	if n := len(l.Services); n > 0 {
		services = make([]*ConsulIngressService, n)
		for i := 0; i < n; i++ {
			services[i] = l.Services[i].Copy()
		}
	}

	return &ConsulIngressListener{
		Port:     l.Port,
		Protocol: l.Protocol,
		Services: services,
	}
}

func (l *ConsulIngressListener) Equals(o *ConsulIngressListener) bool {
	if l == nil || o == nil {
		return l == o
	}

	if l.Port != o.Port {
		return false
	}

	if l.Protocol != o.Protocol {
		return false
	}

	return ingressServicesEqual(l.Services, o.Services)
}

func (l *ConsulIngressListener) Validate() error {
	if l == nil {
		return nil
	}

	if l.Port <= 0 {
		return fmt.Errorf("Consul Ingress Listener requires valid Port")
	}

	protocols := []string{"http", "tcp"}
	if !helper.SliceStringContains(protocols, l.Protocol) {
		return fmt.Errorf(`Consul Ingress Listener requires protocol of "http" or "tcp", got %q`, l.Protocol)
	}

	if len(l.Services) == 0 {
		return fmt.Errorf("Consul Ingress Listener requires one or more services")
	}

	for _, service := range l.Services {
		if err := service.Validate(l.Protocol == "http"); err != nil {
			return err
		}
	}

	return nil
}

func ingressServicesEqual(servicesA, servicesB []*ConsulIngressService) bool {
	if len(servicesA) != len(servicesB) {
		return false
	}

COMPARE: // order does not matter
	for _, serviceA := range servicesA {
		for _, serviceB := range servicesB {
			if serviceA.Equals(serviceB) {
				continue COMPARE
			}
		}
		return false
	}
	return true
}

// ConsulIngressConfigEntry represents the Consul Configuration Entry type for
// an Ingress Gateway.
//
// https://www.consul.io/docs/agent/config-entries/ingress-gateway#available-fields
type ConsulIngressConfigEntry struct {
	// Namespace is not yet supported.
	// Namespace string

	TLS       *ConsulGatewayTLSConfig
	Listeners []*ConsulIngressListener
}

func (e *ConsulIngressConfigEntry) Copy() *ConsulIngressConfigEntry {
	if e == nil {
		return nil
	}

	var listeners []*ConsulIngressListener = nil
	if n := len(e.Listeners); n > 0 {
		listeners = make([]*ConsulIngressListener, n)
		for i := 0; i < n; i++ {
			listeners[i] = e.Listeners[i].Copy()
		}
	}

	return &ConsulIngressConfigEntry{
		TLS:       e.TLS.Copy(),
		Listeners: listeners,
	}
}

func (e *ConsulIngressConfigEntry) Equals(o *ConsulIngressConfigEntry) bool {
	if e == nil || o == nil {
		return e == o
	}

	if !e.TLS.Equals(o.TLS) {
		return false
	}

	return ingressListenersEqual(e.Listeners, o.Listeners)
}

func (e *ConsulIngressConfigEntry) Validate() error {
	if e == nil {
		return nil
	}

	if len(e.Listeners) == 0 {
		return fmt.Errorf("Consul Ingress Gateway requires at least one listener")
	}

	for _, listener := range e.Listeners {
		if err := listener.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func ingressListenersEqual(listenersA, listenersB []*ConsulIngressListener) bool {
	if len(listenersA) != len(listenersB) {
		return false
	}

COMPARE: // order does not matter
	for _, listenerA := range listenersA {
		for _, listenerB := range listenersB {
			if listenerA.Equals(listenerB) {
				continue COMPARE
			}
		}
		return false
	}
	return true
}

type ConsulLinkedService struct {
	Name     string
	CAFile   string
	CertFile string
	KeyFile  string
	SNI      string
}

func (s *ConsulLinkedService) Copy() *ConsulLinkedService {
	if s == nil {
		return nil
	}

	return &ConsulLinkedService{
		Name:     s.Name,
		CAFile:   s.CAFile,
		CertFile: s.CertFile,
		KeyFile:  s.KeyFile,
		SNI:      s.SNI,
	}
}

func (s *ConsulLinkedService) Equals(o *ConsulLinkedService) bool {
	if s == nil || o == nil {
		return s == o
	}

	switch {
	case s.Name != o.Name:
		return false
	case s.CAFile != o.CAFile:
		return false
	case s.CertFile != o.CertFile:
		return false
	case s.KeyFile != o.KeyFile:
		return false
	case s.SNI != o.SNI:
		return false
	}

	return true
}

func (s *ConsulLinkedService) Validate() error {
	if s == nil {
		return nil
	}

	if s.Name == "" {
		return fmt.Errorf("Consul Linked Service requires Name")
	}

	caSet := s.CAFile != ""
	certSet := s.CertFile != ""
	keySet := s.KeyFile != ""
	sniSet := s.SNI != ""

	if (certSet || keySet) && !caSet {
		return fmt.Errorf("Consul Linked Service TLS requires CAFile")
	}

	if certSet != keySet {
		return fmt.Errorf("Consul Linked Service TLS Cert and Key must both be set")
	}

	if sniSet && !caSet {
		return fmt.Errorf("Consul Linked Service TLS SNI requires CAFile")
	}

	return nil
}

func linkedServicesEqual(servicesA, servicesB []*ConsulLinkedService) bool {
	if len(servicesA) != len(servicesB) {
		return false
	}

COMPARE: // order does not matter
	for _, serviceA := range servicesA {
		for _, serviceB := range servicesB {
			if serviceA.Equals(serviceB) {
				continue COMPARE
			}
		}
		return false
	}
	return true
}

type ConsulTerminatingConfigEntry struct {
	// Namespace is not yet supported.
	// Namespace string

	Services []*ConsulLinkedService
}

func (e *ConsulTerminatingConfigEntry) Copy() *ConsulTerminatingConfigEntry {
	if e == nil {
		return nil
	}

	var services []*ConsulLinkedService = nil
	if n := len(e.Services); n > 0 {
		services = make([]*ConsulLinkedService, n)
		for i := 0; i < n; i++ {
			services[i] = e.Services[i].Copy()
		}
	}

	return &ConsulTerminatingConfigEntry{
		Services: services,
	}
}

func (e *ConsulTerminatingConfigEntry) Equals(o *ConsulTerminatingConfigEntry) bool {
	if e == nil || o == nil {
		return e == o
	}

	return linkedServicesEqual(e.Services, o.Services)
}

func (e *ConsulTerminatingConfigEntry) Validate() error {
	if e == nil {
		return nil
	}

	if len(e.Services) == 0 {
		return fmt.Errorf("Consul Terminating Gateway requires at least one service")
	}

	for _, service := range e.Services {
		if err := service.Validate(); err != nil {
			return err
		}
	}

	return nil
}
