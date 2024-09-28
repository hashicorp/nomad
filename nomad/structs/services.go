// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"maps"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/mitchellh/copystructure"
)

const (
	EnvoyBootstrapPath = "${NOMAD_SECRETS_DIR}/envoy_bootstrap.json"

	ServiceCheckHTTP   = "http"
	ServiceCheckTCP    = "tcp"
	ServiceCheckScript = "script"
	ServiceCheckGRPC   = "grpc"

	OnUpdateRequireHealthy = "require_healthy"
	OnUpdateIgnoreWarn     = "ignore_warnings"
	OnUpdateIgnore         = "ignore"

	// minCheckInterval is the minimum check interval permitted.  Consul
	// currently has its MinInterval set to 1s.  Mirror that here for
	// consistency.
	minCheckInterval = 1 * time.Second

	// minCheckTimeout is the minimum check timeout permitted for Consul
	// script TTL checks.
	minCheckTimeout = 1 * time.Second
)

// ServiceCheck represents a Nomad or Consul service health check.
//
// The fields available depend on the service provider the check is being
// registered into.
type ServiceCheck struct {
	Name                   string              // Name of the check, defaults to a generated label
	Type                   string              // Type of the check - tcp, http, docker and script
	Command                string              // Command is the command to run for script checks
	Args                   []string            // Args is a list of arguments for script checks
	Path                   string              // path of the health check url for http type check
	Protocol               string              // Protocol to use if check is http, defaults to http
	PortLabel              string              // The port to use for tcp/http checks
	Expose                 bool                // Whether to have Envoy expose the check path (connect-enabled group-services only)
	AddressMode            string              // Must be empty, "alloc", "host", or "driver"
	Interval               time.Duration       // Interval of the check
	Timeout                time.Duration       // Timeout of the response from the check before consul fails the check
	InitialStatus          string              // Initial status of the check
	Notes                  string              // Specifies arbitrary information for humans. This is not used by Consul internally
	TLSServerName          string              // ServerName to use for SNI and TLS verification when (Type=https and Protocol=https) or (Type=grpc and GRPCUseTLS=true)
	TLSSkipVerify          bool                // Skip TLS verification when (type=https and Protocol=https) or (type=grpc and grpc_use_tls=true)
	Method                 string              // HTTP Method to use (GET by default)
	Header                 map[string][]string // HTTP Headers for Consul to set when making HTTP checks
	CheckRestart           *CheckRestart       // If and when a task should be restarted based on checks
	GRPCService            string              // Service for GRPC checks
	GRPCUseTLS             bool                // Whether or not to use TLS for GRPC checks
	TaskName               string              // What task to execute this check in
	SuccessBeforePassing   int                 // Number of consecutive successes required before considered healthy
	FailuresBeforeCritical int                 // Number of consecutive failures required before considered unhealthy
	FailuresBeforeWarning  int                 // Number of consecutive failures required before showing warning
	Body                   string              // Body to use in HTTP check
	OnUpdate               string
}

// IsReadiness returns whether the configuration of the ServiceCheck is effectively
// a readiness check - i.e. check failures do not affect a deployment.
func (sc *ServiceCheck) IsReadiness() bool {
	return sc != nil && sc.OnUpdate == OnUpdateIgnore
}

// Copy the block recursively. Returns nil if nil.
func (sc *ServiceCheck) Copy() *ServiceCheck {
	if sc == nil {
		return nil
	}
	nsc := new(ServiceCheck)
	*nsc = *sc
	nsc.Args = slices.Clone(sc.Args)
	nsc.Header = helper.CopyMapOfSlice(sc.Header)
	nsc.CheckRestart = sc.CheckRestart.Copy()
	return nsc
}

// Equal returns true if the structs are recursively equal.
func (sc *ServiceCheck) Equal(o *ServiceCheck) bool {
	if sc == nil || o == nil {
		return sc == o
	}

	if sc.Name != o.Name {
		return false
	}

	if sc.AddressMode != o.AddressMode {
		return false
	}

	if !helper.SliceSetEq(sc.Args, o.Args) {
		return false
	}

	if !sc.CheckRestart.Equal(o.CheckRestart) {
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

	if sc.FailuresBeforeWarning != o.FailuresBeforeWarning {
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

	if sc.Notes != o.Notes {
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

	if sc.PortLabel != o.PortLabel {
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

	if sc.TLSServerName != o.TLSServerName {
		return false
	}

	if sc.Timeout != o.Timeout {
		return false
	}

	if sc.Type != o.Type {
		return false
	}

	if sc.Body != o.Body {
		return false
	}

	if sc.OnUpdate != o.OnUpdate {
		return false
	}

	return true
}

func (sc *ServiceCheck) Canonicalize(serviceName, taskName string) {
	// Ensure empty maps/slices are treated as null to avoid scheduling
	// issues when using DeepEquals.
	if len(sc.Args) == 0 {
		sc.Args = nil
	}

	// Ensure empty slices are nil
	if len(sc.Header) == 0 {
		sc.Header = nil
	} else {
		for k, v := range sc.Header {
			if len(v) == 0 {
				sc.Header[k] = nil
			}
		}
	}

	// Ensure a default name for the check
	if sc.Name == "" {
		sc.Name = fmt.Sprintf("service: %q check", serviceName)
	}

	// Set task name if not already set
	if sc.TaskName == "" && taskName != "group" {
		sc.TaskName = taskName
	}

	// Ensure OnUpdate defaults to require_healthy (i.e. healthiness check)
	if sc.OnUpdate == "" {
		sc.OnUpdate = OnUpdateRequireHealthy
	}
}

// validateCommon validates the parts of ServiceCheck shared across providers.
func (sc *ServiceCheck) validateCommon(allowableTypes []string) error {
	// validate the type is allowable (different between nomad, consul checks)
	checkType := strings.ToLower(sc.Type)
	if !slices.Contains(allowableTypes, checkType) {
		s := strings.Join(allowableTypes, ", ")
		return fmt.Errorf(`invalid check type (%q), must be one of %s`, checkType, s)
	}

	// validate specific check types
	switch checkType {
	case ServiceCheckHTTP:
		if sc.Path == "" {
			return fmt.Errorf("http type must have http path")
		}
		checkPath, pathErr := url.Parse(sc.Path)
		if pathErr != nil {
			return fmt.Errorf("http type must have valid http path")
		}
		if checkPath.IsAbs() {
			return fmt.Errorf("http type must have relative http path")
		}
	case ServiceCheckScript:
		if sc.Command == "" {
			return fmt.Errorf("script type must have a valid script path")
		}
	}

	// validate interval
	if sc.Interval == 0 {
		return fmt.Errorf("missing required value interval. Interval cannot be less than %v", minCheckInterval)
	} else if sc.Interval < minCheckInterval {
		return fmt.Errorf("interval (%v) cannot be lower than %v", sc.Interval, minCheckInterval)
	}

	// validate timeout
	if sc.Timeout == 0 {
		return fmt.Errorf("missing required value timeout. Timeout cannot be less than %v", minCheckInterval)
	} else if sc.Timeout < minCheckTimeout {
		return fmt.Errorf("timeout (%v) is lower than required minimum timeout %v", sc.Timeout, minCheckInterval)
	}

	// validate the initial status
	switch sc.InitialStatus {
	case "":
	case api.HealthPassing:
	case api.HealthWarning:
	case api.HealthCritical:
	default:
		return fmt.Errorf(`invalid initial check state (%s), must be one of %q, %q, %q or empty`, sc.InitialStatus, api.HealthPassing, api.HealthWarning, api.HealthCritical)
	}

	// validate address_mode
	switch sc.AddressMode {
	case "", AddressModeHost, AddressModeDriver, AddressModeAlloc:
		// Ok
	case AddressModeAuto:
		return fmt.Errorf("invalid address_mode %q - %s only valid for services", sc.AddressMode, AddressModeAuto)
	default:
		return fmt.Errorf("invalid address_mode %q", sc.AddressMode)
	}

	// validate on_update
	switch sc.OnUpdate {
	case "", OnUpdateIgnore, OnUpdateRequireHealthy, OnUpdateIgnoreWarn:
		// OK
	default:
		return fmt.Errorf("on_update must be %q, %q, or %q; got %q", OnUpdateRequireHealthy, OnUpdateIgnoreWarn, OnUpdateIgnore, sc.OnUpdate)
	}

	// validate check_restart and on_update do not conflict
	if sc.CheckRestart != nil {
		// CheckRestart and OnUpdate Ignore are incompatible If OnUpdate treats
		// an error has healthy, and the deployment succeeds followed by check
		// restart restarting failing checks, the deployment is left in an odd
		// state
		if sc.OnUpdate == OnUpdateIgnore {
			return fmt.Errorf("on_update value %q is not compatible with check_restart", sc.OnUpdate)
		}
		// CheckRestart IgnoreWarnings must be true if a check has defined OnUpdate
		// ignore_warnings
		if !sc.CheckRestart.IgnoreWarnings && sc.OnUpdate == OnUpdateIgnoreWarn {
			return fmt.Errorf("on_update value %q not supported with check_restart ignore_warnings value %q", sc.OnUpdate, strconv.FormatBool(sc.CheckRestart.IgnoreWarnings))
		}
	}

	// validate check_restart
	if err := sc.CheckRestart.Validate(); err != nil {
		return err
	}

	return nil
}

// validate a Service's ServiceCheck in the context of the Nomad provider.
func (sc *ServiceCheck) validateNomad() error {
	allowable := []string{ServiceCheckTCP, ServiceCheckHTTP}
	if err := sc.validateCommon(allowable); err != nil {
		return err
	}

	// expose is connect (consul) specific
	if sc.Expose {
		return errors.New("expose may only be set for Consul service checks")
	}

	// nomad checks do not have warnings
	if sc.OnUpdate == OnUpdateIgnoreWarn {
		return errors.New("on_update may only be set to ignore_warnings for Consul service checks")
	}

	// below are temporary limitations on checks in nomad
	// https://github.com/hashicorp/team-nomad/issues/354

	// check_restart.ignore_warnings is not a thing in Nomad (which has no warnings in checks)
	if sc.CheckRestart != nil {
		if sc.CheckRestart.IgnoreWarnings {
			return errors.New("ignore_warnings on check_restart only supported for Consul service checks")
		}
	}

	// address_mode="driver" not yet supported on nomad
	if sc.AddressMode == "driver" {
		return errors.New("address_mode = driver may only be set for Consul service checks")
	}

	if sc.Type == "http" {
		if sc.Method != "" && !helper.IsMethodHTTP(sc.Method) {
			return fmt.Errorf("method type %q not supported in Nomad http check", sc.Method)
		}
	}

	// success_before_passing is consul only
	if sc.SuccessBeforePassing != 0 {
		return errors.New("success_before_passing may only be set for Consul service checks")
	}

	// failures_before_critical is consul only
	if sc.FailuresBeforeCritical != 0 {
		return errors.New("failures_before_critical may only be set for Consul service checks")
	}

	// failures_before_warning is consul only
	if sc.FailuresBeforeWarning != 0 {
		return errors.New("failures_before_warning may only be set for Consul service checks")
	}

	// tls_server_name is consul only
	if sc.TLSServerName != "" {
		return errors.New("tls_server_name may only be set for Consul service checks")
	}

	// tls_skip_verify is consul only
	if sc.TLSSkipVerify {
		return errors.New("tls_skip_verify may only be set for Consul service checks")
	}

	return nil
}

// validate a Service's ServiceCheck in the context of the Consul provider.
func (sc *ServiceCheck) validateConsul() error {
	allowable := []string{ServiceCheckGRPC, ServiceCheckTCP, ServiceCheckHTTP, ServiceCheckScript}
	if err := sc.validateCommon(allowable); err != nil {
		return err
	}

	checkType := strings.ToLower(sc.Type)

	// Note that we cannot completely validate the Expose field yet - we do not
	// know whether this ServiceCheck belongs to a connect-enabled group-service.
	// Instead, such validation will happen in a job admission controller.
	//
	// Consul only.
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
	//
	// Consul only.
	passFailCheckTypes := []string{"tcp", "http", "grpc"}

	if sc.SuccessBeforePassing < 0 {
		return fmt.Errorf("success_before_passing must be non-negative")
	} else if sc.SuccessBeforePassing > 0 && !slices.Contains(passFailCheckTypes, sc.Type) {
		return fmt.Errorf("success_before_passing not supported for check of type %q", sc.Type)
	}

	if sc.FailuresBeforeCritical < 0 {
		return fmt.Errorf("failures_before_critical must be non-negative")
	} else if sc.FailuresBeforeCritical > 0 && !slices.Contains(passFailCheckTypes, sc.Type) {
		return fmt.Errorf("failures_before_critical not supported for check of type %q", sc.Type)
	}

	if sc.FailuresBeforeWarning < 0 {
		return fmt.Errorf("failures_before_warning must be non-negative")
	} else if sc.FailuresBeforeWarning > 0 && !slices.Contains(passFailCheckTypes, sc.Type) {
		return fmt.Errorf("failures_before_warning not supported for check of type %q", sc.Type)
	}

	// Arbitrary value, we could bump it if needed
	if len(sc.Notes) > 255 {
		return fmt.Errorf("notes must not be longer than 255 characters")
	}

	return nil
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
	hashString(h, sc.Body)
	hashString(h, sc.OnUpdate)
	hashString(h, sc.Notes)

	// use name "true" to maintain ID stability
	hashBool(h, sc.TLSSkipVerify, "true")

	// Only include TLSServerName if set to maintain ID stability with Nomad <1.6.0
	hashStringIfNonEmpty(h, sc.TLSServerName)

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
	hashIntIfNonZero(h, "failures-before-warning", sc.FailuresBeforeWarning)

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

func hashDuration(h hash.Hash, dur time.Duration) {
	_ = binary.Write(h, binary.LittleEndian, dur)
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

	// ServiceProviderConsul is the default service provider and the way Nomad
	// worked before native service discovery.
	ServiceProviderConsul = "consul"

	// ServiceProviderNomad is the native service discovery provider. At the
	// time of writing, there are a number of restrictions around its
	// functionality and use.
	ServiceProviderNomad = "nomad"
)

// Service represents a Consul service definition
type Service struct {
	// Name of the service registered with Consul. Consul defaults the
	// Name to ServiceID if not specified.  The Name if specified is used
	// as one of the seed values when generating a Consul ServiceID.
	Name string

	// Name of the Task associated with this service.
	// Group services do not have a task name, unless they are a connect native
	// service specifying the task implementing the service.
	// Task-level services automatically have the task name plumbed through
	// down to checks for convenience.
	TaskName string

	// PortLabel is either the numeric port number or the `host:port`.
	// To specify the port number using the host's Consul Advertise
	// address, specify an empty host in the PortLabel (e.g. `:port`).
	PortLabel string

	// AddressMode specifies how the address in service registration is
	// determined. Must be "auto" (default), "host", "driver", or "alloc".
	AddressMode string

	// Address enables explicitly setting a custom address to use in service
	// registration. AddressMode must be "auto" if Address is set.
	Address string

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

	// The values to set for tagged_addresses in Consul service registration.
	// Does not affect Nomad networking, these are for Consul service discovery.
	TaggedAddresses map[string]string

	// The consul namespace in which this service will be registered. Namespace
	// at the service.check level is not part of the Nomad API - it must be
	// set at the job or group level. This field is managed internally so
	// that Hash can work correctly.
	Namespace string

	// OnUpdate Specifies how the service and its checks should be evaluated
	// during an update
	OnUpdate string

	// Provider dictates which service discovery provider to use. This can be
	// either ServiceProviderConsul or ServiceProviderNomad and defaults to the former when
	// left empty by the operator.
	Provider string

	// Consul Cluster (by name) to send API requests to
	Cluster string

	// Identity is a field populated automatically by the job mutating hook.
	// Its name will be `consul-service/${service_name}`, and its contents will
	// match the server's `consul.service_identity` configuration block.
	Identity *WorkloadIdentity
}

// Copy the block recursively. Returns nil if nil.
func (s *Service) Copy() *Service {
	if s == nil {
		return nil
	}
	ns := new(Service)
	*ns = *s
	ns.Tags = slices.Clone(ns.Tags)
	ns.CanaryTags = slices.Clone(ns.CanaryTags)

	if s.Checks != nil {
		checks := make([]*ServiceCheck, len(ns.Checks))
		for i, c := range ns.Checks {
			checks[i] = c.Copy()
		}
		ns.Checks = checks
	}

	ns.Connect = s.Connect.Copy()

	ns.Meta = maps.Clone(s.Meta)
	ns.CanaryMeta = maps.Clone(s.CanaryMeta)
	ns.TaggedAddresses = maps.Clone(s.TaggedAddresses)

	ns.Identity = s.Identity.Copy()

	return ns
}

// Canonicalize interpolates values of Job, Task Group and Task in the Service
// Name. This also generates check names, service id and check ids.
func (s *Service) Canonicalize(job, taskGroup, task, jobNamespace string) {
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
	if len(s.TaggedAddresses) == 0 {
		s.TaggedAddresses = nil
	}

	// Set the task name if not already set
	if s.TaskName == "" && task != "group" {
		s.TaskName = task
	}

	s.Name = args.ReplaceEnv(s.Name, map[string]string{
		"JOB":       job,
		"TASKGROUP": taskGroup,
		"TASK":      task,
		"BASE":      fmt.Sprintf("%s-%s-%s", job, taskGroup, task),
	})

	for _, check := range s.Checks {
		check.Canonicalize(s.Name, s.TaskName)
	}

	// Set the provider to its default value. The value of consul ensures this
	// new feature and parameter behaves in the same manner a previous versions
	// which did not include this.
	if s.Provider == "" {
		s.Provider = ServiceProviderConsul
	}

	// Consul API returns "default" whether the namespace is empty or set as
	// such, so we coerce our copy of the service to be the same if using the
	// consul provider.
	//
	// When using ServiceProviderNomad, set the namespace to that of the job. This
	// makes modifications and diffs on the service correct.
	if s.Namespace == "" && s.Provider == ServiceProviderConsul {
		s.Namespace = "default"
	} else if s.Provider == ServiceProviderNomad {
		s.Namespace = jobNamespace
	}
}

// Warnings returns a list of warnings that may be from dubious settings or
// deprecation warnings.
func (s *Service) Warnings() error {
	var mErr *multierror.Error

	if s.Identity != nil {
		if err := s.Identity.Warnings(); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	return mErr.ErrorOrNil()
}

// Validate checks if the Service definition is valid
func (s *Service) Validate() error {
	var mErr multierror.Error

	// Ensure the service name is valid per the below RFCs but make an exception
	// for our interpolation syntax by first stripping any environment variables from the name

	serviceNameStripped := args.ReplaceEnvWithPlaceHolder(s.Name, "ENV-VAR")

	if err := s.ValidateName(serviceNameStripped); err != nil {
		// Log actual service name, not the stripped version.
		mErr.Errors = append(mErr.Errors, fmt.Errorf("%v: %q", err, s.Name))
	}

	switch s.AddressMode {
	case "", AddressModeAuto:
	case AddressModeHost, AddressModeDriver, AddressModeAlloc:
		if s.Address != "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Service address_mode must be %q if address is set", AddressModeAuto))
		}
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Service address_mode must be %q, %q, or %q; not %q", AddressModeAuto, AddressModeHost, AddressModeDriver, s.AddressMode))
	}

	switch s.OnUpdate {
	case "", OnUpdateIgnore, OnUpdateRequireHealthy, OnUpdateIgnoreWarn:
		// OK
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Service on_update must be %q, %q, or %q; not %q", OnUpdateRequireHealthy, OnUpdateIgnoreWarn, OnUpdateIgnore, s.OnUpdate))
	}

	// Up until this point, all service validation has been independent of the
	// provider. From this point on, we have different validation paths. We can
	// also catch an incorrect provider parameter.
	switch s.Provider {
	case ServiceProviderConsul:
		s.validateConsulService(&mErr)
	case ServiceProviderNomad:
		s.validateNomadService(&mErr)
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Service provider must be %q, or %q; not %q",
			ServiceProviderConsul, ServiceProviderNomad, s.Provider))
	}

	if err := s.validateIdentity(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

// MakeUniqueIdentityName returns a service identity name consisting of: task
// name, service name and service port label.
func (s *Service) MakeUniqueIdentityName() string {
	prefix := ConsulServiceIdentityNamePrefix
	if s.Provider == ServiceProviderNomad {
		prefix = "nomad-service"
	}
	if s.TaskName != "" {
		return fmt.Sprintf("%s_%v-%v-%v", prefix, s.TaskName, s.Name, s.PortLabel)
	}
	return fmt.Sprintf("%s_%v-%v", prefix, s.Name, s.PortLabel)
}

type envReplacer func(string) string

// IdentityHandle returns a WorkloadIdentityHandle which is a pair of service
// identity name and service name.
func (s *Service) IdentityHandle(replace envReplacer) *WIHandle {
	if s.Identity != nil {
		wi := &WIHandle{
			IdentityName:       s.Identity.Name,
			WorkloadIdentifier: s.Name,
			WorkloadType:       WorkloadTypeService,
		}
		if replace != nil {
			wi.InterpolatedWorkloadIdentifier = replace(s.Name)
		}
		return wi
	}
	return nil
}

func (s *Service) validateCheckPort(c *ServiceCheck) error {
	if s.PortLabel == "" && c.PortLabel == "" && c.RequiresPort() {
		return fmt.Errorf("Check %s invalid: check requires a port but neither check nor service %+q have a port", c.Name, s.Name)
	}
	return nil
}

// validateConsulService performs validation on a service which is using the
// consul provider.
func (s *Service) validateConsulService(mErr *multierror.Error) {
	// check checks
	for _, c := range s.Checks {
		// validat ethe check port
		if err := s.validateCheckPort(c); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}

		// TCP checks against a Consul Connect enabled service are not supported
		// due to the service being bound to the loopback interface inside the
		// network namespace
		if c.Type == ServiceCheckTCP && s.Connect != nil && s.Connect.SidecarService != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Check %s invalid: tcp checks are not valid for Connect enabled services", c.Name))
			continue
		}

		// validate the consul check
		if err := c.validateConsul(); err != nil {
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
}

// validateNomadService performs validation on a service which is using the
// nomad provider.
func (s *Service) validateNomadService(mErr *multierror.Error) {
	// check checks
	for _, c := range s.Checks {
		// validate the check port
		if err := s.validateCheckPort(c); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}

		// validate the nomad check
		if err := c.validateNomad(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	// Services using the Nomad provider do not support Consul connect.
	if s.Connect != nil {
		mErr.Errors = append(mErr.Errors, errors.New("Service with provider nomad cannot include Connect blocks"))
	}
}

// validateIdentity performs validation on workload identity field populated by
// the job mutating hook
func (s *Service) validateIdentity() error {
	if s.Identity == nil {
		return nil
	}

	if len(s.Identity.Audience) == 0 {
		return fmt.Errorf("Service identity must provide at least one target aud value")
	}

	return nil
}

// ValidateName checks if the service Name is valid and should be called after
// the name has been interpolated
func (s *Service) ValidateName(name string) error {
	// Ensure the service name is valid per RFC-952 §1
	// (https://tools.ietf.org/html/rfc952), RFC-1123 §2.1
	// (https://tools.ietf.org/html/rfc1123), and RFC-2782
	// (https://tools.ietf.org/html/rfc2782).
	//  This validation is enforced on Nomad, but not on Consul, however if
	//  consul-template is being used, service names with dots in them wont be
	//  admissible.
	re := regexp.MustCompile(`^(?i:[a-z0-9]|[a-z0-9][a-z0-9\-]{0,61}[a-z0-9])$`)
	if !re.MatchString(name) {
		return fmt.Errorf("Service name must be valid per RFC 1123 and can contain only alphanumeric characters or dashes and must be no longer than 63 characters")
	}
	return nil
}

// Hash returns a base32 encoded hash of a Service's contents excluding checks
// as they're hashed independently and the provider in order to not cause churn
// during cluster upgrades.
func (s *Service) Hash(allocID, taskName string, canary bool) string {
	h := sha1.New()
	hashString(h, allocID)
	hashString(h, taskName)
	hashString(h, s.Name)
	hashString(h, s.PortLabel)
	hashString(h, s.AddressMode)
	hashString(h, s.Address)
	hashTags(h, s.Tags)
	hashTags(h, s.CanaryTags)
	hashBool(h, canary, "Canary")
	hashBool(h, s.EnableTagOverride, "ETO")
	hashMeta(h, s.Meta)
	hashMeta(h, s.CanaryMeta)
	hashMeta(h, s.TaggedAddresses)
	hashConnect(h, s.Connect)
	hashString(h, s.OnUpdate)
	hashString(h, s.Namespace)
	hashIdentity(h, s.Identity)

	// Don't hash the provider parameter, so we don't cause churn of all
	// registered services when upgrading Nomad versions. The provider is not
	// used at the level the hash is and therefore is not needed to tell
	// whether the service has changed.

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
			hashTProxy(h, p.TransparentProxy)
			for _, upstream := range p.Upstreams {
				hashString(h, upstream.DestinationName)
				hashString(h, upstream.DestinationNamespace)
				hashString(h, strconv.Itoa(upstream.LocalBindPort))
				hashStringIfNonEmpty(h, upstream.Datacenter)
				hashStringIfNonEmpty(h, upstream.LocalBindAddress)
				hashString(h, upstream.DestinationPeer)
				hashString(h, upstream.DestinationPartition)
				hashString(h, upstream.DestinationType)
				hashString(h, upstream.LocalBindSocketPath)
				hashString(h, upstream.LocalBindSocketMode)
				hashConfig(h, upstream.Config)
			}
		}
	}
}

func hashIdentity(h hash.Hash, identity *WorkloadIdentity) {
	if identity != nil {
		hashString(h, identity.Name)
		hashAud(h, identity.Audience)
		hashBool(h, identity.Env, "Env")
		hashBool(h, identity.File, "File")
		hashString(h, identity.ServiceName)
	}
}

func hashAud(h hash.Hash, aud []string) {
	for _, a := range aud {
		hashString(h, a)
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

func hashTProxy(h hash.Hash, tp *ConsulTransparentProxy) {
	if tp == nil {
		return
	}

	hashStringIfNonEmpty(h, tp.UID)
	hashIntIfNonZero(h, "OutboundPort", int(tp.OutboundPort))
	hashTags(h, tp.ExcludeInboundPorts)
	for _, port := range tp.ExcludeOutboundPorts {
		hashIntIfNonZero(h, "ExcludeOutboundPorts", int(port))
	}
	hashTags(h, tp.ExcludeOutboundCIDRs)
	hashTags(h, tp.ExcludeUIDs)
	hashBool(h, tp.NoDNS, "NoDNS")
}

// Equal returns true if the structs are recursively equal.
func (s *Service) Equal(o *Service) bool {
	if s == nil || o == nil {
		return s == o
	}

	if s.Provider != o.Provider {
		return false
	}

	if s.Cluster != o.Cluster {
		return false
	}

	if s.Namespace != o.Namespace {
		return false
	}

	if s.AddressMode != o.AddressMode {
		return false
	}

	if s.Address != o.Address {
		return false
	}

	if s.OnUpdate != o.OnUpdate {
		return false
	}

	if !helper.SliceSetEq(s.CanaryTags, o.CanaryTags) {
		return false
	}

	if !helper.ElementsEqual(s.Checks, o.Checks) {
		return false
	}

	if !s.Connect.Equal(o.Connect) {
		return false
	}

	if s.Name != o.Name {
		return false
	}

	if s.PortLabel != o.PortLabel {
		return false
	}

	if !maps.Equal(s.Meta, o.Meta) {
		return false
	}

	if !maps.Equal(s.CanaryMeta, o.CanaryMeta) {
		return false
	}

	if !maps.Equal(s.TaggedAddresses, o.TaggedAddresses) {
		return false
	}

	if !helper.SliceSetEq(s.Tags, o.Tags) {
		return false
	}

	if s.EnableTagOverride != o.EnableTagOverride {
		return false
	}

	if !s.Identity.Equal(o.Identity) {
		return false
	}

	return true
}

func (s *Service) IsConsul() bool {
	return s.Provider == ServiceProviderConsul || s.Provider == ""
}

// ConsulConnect represents a Consul Connect jobspec block.
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

// Copy the block recursively. Returns nil if nil.
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

// Equal returns true if the connect blocks are deeply equal.
func (c *ConsulConnect) Equal(o *ConsulConnect) bool {
	if c == nil || o == nil {
		return c == o
	}

	if c.Native != o.Native {
		return false
	}

	if !c.SidecarService.Equal(o.SidecarService) {
		return false
	}

	if !c.SidecarTask.Equal(o.SidecarTask) {
		return false
	}

	if !c.Gateway.Equal(o.Gateway) {
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

// IsCustomizedTLS checks if the service customizes ingress tls config.
func (c *ConsulConnect) IsCustomizedTLS() bool {
	return c.IsIngress() && c.Gateway.Ingress.TLS != nil &&
		(c.Gateway.Ingress.TLS.TLSMinVersion != "" ||
			c.Gateway.Ingress.TLS.TLSMaxVersion != "" ||
			len(c.Gateway.Ingress.TLS.CipherSuites) != 0)
}

func (c *ConsulConnect) IsMesh() bool {
	return c.IsGateway() && c.Gateway.Mesh != nil
}

// HasTransparentProxy checks if a service with a Connect sidecar has a
// transparent proxy configuration
func (c *ConsulConnect) HasTransparentProxy() bool {
	return c.HasSidecar() &&
		c.SidecarService.Proxy != nil &&
		c.SidecarService.Proxy.TransparentProxy != nil
}

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
		if c.HasTransparentProxy() {
			if err := c.SidecarService.Proxy.TransparentProxy.Validate(); err != nil {
				return err
			}
		}
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

	// Checking against the surrounding task group is validated up at the
	// service level or job endpint connect validation hook

	return nil
}

// ConsulSidecarService represents a Consul Connect SidecarService jobspec
// block.
type ConsulSidecarService struct {
	// Tags are optional service tags that get registered with the sidecar service
	// in Consul. If unset, the sidecar service inherits the parent service tags.
	Tags []string

	// Port is the service's port that the sidecar will connect to. May be
	// a port label or a literal port number.
	Port string

	// Proxy block defining the sidecar proxy configuration.
	Proxy *ConsulProxy

	// DisableDefaultTCPCheck, if true, instructs Nomad to avoid setting a
	// default TCP check for the sidecar service.
	DisableDefaultTCPCheck bool

	// Meta specifies arbitrary KV metadata linked to the sidecar service.
	Meta map[string]string
}

// HasUpstreams checks if the sidecar service has any upstreams configured
func (s *ConsulSidecarService) HasUpstreams() bool {
	return s != nil && s.Proxy != nil && len(s.Proxy.Upstreams) > 0
}

// Copy the block recursively. Returns nil if nil.
func (s *ConsulSidecarService) Copy() *ConsulSidecarService {
	if s == nil {
		return nil
	}
	return &ConsulSidecarService{
		Tags:                   slices.Clone(s.Tags),
		Port:                   s.Port,
		Proxy:                  s.Proxy.Copy(),
		DisableDefaultTCPCheck: s.DisableDefaultTCPCheck,
		Meta:                   maps.Clone(s.Meta),
	}
}

// Equal returns true if the structs are recursively equal.
func (s *ConsulSidecarService) Equal(o *ConsulSidecarService) bool {
	if s == nil || o == nil {
		return s == o
	}

	if s.Port != o.Port {
		return false
	}

	if s.DisableDefaultTCPCheck != o.DisableDefaultTCPCheck {
		return false
	}

	if !helper.SliceSetEq(s.Tags, o.Tags) {
		return false
	}

	if !maps.Equal(s.Meta, o.Meta) {
		return false
	}

	return s.Proxy.Equal(o.Proxy)
}

// SidecarTask represents a subset of Task fields that are able to be overridden
// from the sidecar_task block
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

	// VolumeMounts is a list of Volume name <-> mount configurations that will be
	// attached to this task.
	VolumeMounts []*VolumeMount
}

func (t *SidecarTask) Equal(o *SidecarTask) bool {
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

	// task config, use opaque maps equal
	if !helper.OpaqueMapsEqual(t.Config, o.Config) {
		return false
	}

	if !maps.Equal(t.Env, o.Env) {
		return false
	}

	if !t.Resources.Equal(o.Resources) {
		return false
	}

	if !maps.Equal(t.Meta, o.Meta) {
		return false
	}

	if !pointer.Eq(t.KillTimeout, o.KillTimeout) {
		return false
	}

	if !t.LogConfig.Equal(o.LogConfig) {
		return false
	}

	if !pointer.Eq(t.ShutdownDelay, o.ShutdownDelay) {
		return false
	}

	if t.KillSignal != o.KillSignal {
		return false
	}

	if !slices.EqualFunc(t.VolumeMounts, o.VolumeMounts,
		func(tVM, oVM *VolumeMount) bool { return tVM.Equal(oVM) }) {
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
	nt.Env = maps.Clone(nt.Env)

	nt.Resources = nt.Resources.Copy()
	nt.LogConfig = nt.LogConfig.Copy()
	nt.Meta = maps.Clone(nt.Meta)

	if i, err := copystructure.Copy(nt.Config); err != nil {
		panic(err.Error())
	} else {
		nt.Config = i.(map[string]interface{})
	}

	if t.KillTimeout != nil {
		nt.KillTimeout = pointer.Of(*t.KillTimeout)
	}

	if t.ShutdownDelay != nil {
		nt.ShutdownDelay = pointer.Of(*t.ShutdownDelay)
	}

	nt.VolumeMounts = CopySliceVolumeMount(t.VolumeMounts)

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

	if t.VolumeMounts != nil {
		task.VolumeMounts = t.VolumeMounts
	}
}

// ConsulProxy represents a Consul Connect sidecar proxy jobspec block.
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

	// Expose configures the consul proxy.expose block to "open up" endpoints
	// used by task-group level service checks using HTTP or gRPC protocols.
	Expose *ConsulExposeConfig

	// TransparentProxy configures the Envoy sidecar to use "transparent
	// proxying", which creates IP tables rules inside the network namespace to
	// ensure traffic flows thru the Envoy proxy
	TransparentProxy *ConsulTransparentProxy

	// Config is a proxy configuration. It is opaque to Nomad and passed
	// directly to Consul.
	Config map[string]interface{}
}

// Copy the block recursively. Returns nil if nil.
func (p *ConsulProxy) Copy() *ConsulProxy {
	if p == nil {
		return nil
	}

	return &ConsulProxy{
		LocalServiceAddress: p.LocalServiceAddress,
		LocalServicePort:    p.LocalServicePort,
		Expose:              p.Expose.Copy(),
		Upstreams:           slices.Clone(p.Upstreams),
		TransparentProxy:    p.TransparentProxy.Copy(),
		Config:              maps.Clone(p.Config),
	}
}

// Equal returns true if the structs are recursively equal.
func (p *ConsulProxy) Equal(o *ConsulProxy) bool {
	if p == nil || o == nil {
		return p == o
	}

	if p.LocalServiceAddress != o.LocalServiceAddress {
		return false
	}

	if p.LocalServicePort != o.LocalServicePort {
		return false
	}

	if !p.Expose.Equal(o.Expose) {
		return false
	}

	if !upstreamsEquals(p.Upstreams, o.Upstreams) {
		return false
	}

	if !p.TransparentProxy.Equal(o.TransparentProxy) {
		return false
	}

	// envoy config, use reflect
	if !reflect.DeepEqual(p.Config, o.Config) {
		return false
	}

	return true
}

// ConsulMeshGateway is used to configure mesh gateway usage when connecting to
// a connect upstream in another datacenter.
type ConsulMeshGateway struct {
	// Mode configures how an upstream should be accessed with regard to using
	// mesh gateways.
	//
	// local - the connect proxy makes outbound connections through mesh gateway
	// originating in the same datacenter.
	//
	// remote - the connect proxy makes outbound connections to a mesh gateway
	// in the destination datacenter.
	//
	// none (default) - no mesh gateway is used, the proxy makes outbound connections
	// directly to destination services.
	//
	// https://www.consul.io/docs/connect/gateways/mesh-gateway#modes-of-operation
	Mode string
}

func (c *ConsulMeshGateway) Copy() ConsulMeshGateway {
	return ConsulMeshGateway{
		Mode: c.Mode,
	}
}

func (c *ConsulMeshGateway) Equal(o ConsulMeshGateway) bool {
	return c.Mode == o.Mode
}

func (c *ConsulMeshGateway) Validate() error {
	if c == nil {
		return nil
	}

	switch c.Mode {
	case "local", "remote", "none":
		return nil
	default:
		return fmt.Errorf("Connect mesh_gateway mode %q not supported", c.Mode)
	}
}

// ConsulUpstream represents a Consul Connect upstream jobspec block.
type ConsulUpstream struct {
	// DestinationName is the name of the upstream service.
	DestinationName string

	// DestinationNamespace is the namespace of the upstream service.
	DestinationNamespace string

	// DestinationNamespace is the admin partition of the upstream service.
	DestinationPartition string

	// DestinationPeer the destination service address
	DestinationPeer string

	// DestinationType is the type of destination. It can be an IP address,
	// a DNS hostname, or a service name.
	DestinationType string

	// LocalBindPort is the port the proxy will receive connections for the
	// upstream on.
	LocalBindPort int

	// Datacenter is the datacenter in which to issue the discovery query to.
	Datacenter string

	// LocalBindAddress is the address the proxy will receive connections for the
	// upstream on.
	LocalBindAddress string

	// LocalBindSocketPath is the path of the local socket file that will be used
	// to connect to the destination service
	LocalBindSocketPath string

	// LocalBindSocketMode defines access permissions to the local socket file
	LocalBindSocketMode string

	// MeshGateway is the optional configuration of the mesh gateway for this
	// upstream to use.
	MeshGateway ConsulMeshGateway

	// Config is an upstream configuration. It is opaque to Nomad and passed
	// directly to Consul.
	Config map[string]any
}

// Equal returns true if the structs are recursively equal.
func (u *ConsulUpstream) Equal(o *ConsulUpstream) bool {
	if u == nil || o == nil {
		return u == o
	}
	switch {
	case u.DestinationName != o.DestinationName:
		return false
	case u.DestinationNamespace != o.DestinationNamespace:
		return false
	case u.DestinationPeer != o.DestinationPeer:
		return false
	case u.DestinationPartition != o.DestinationPartition:
		return false
	case u.DestinationType != o.DestinationType:
		return false
	case u.LocalBindPort != o.LocalBindPort:
		return false
	case u.LocalBindSocketPath != o.LocalBindSocketPath:
		return false
	case u.LocalBindSocketMode != o.LocalBindSocketMode:
		return false
	case u.Datacenter != o.Datacenter:
		return false
	case u.LocalBindAddress != o.LocalBindAddress:
		return false
	case !u.MeshGateway.Equal(o.MeshGateway):
		return false
	case !reflect.DeepEqual(u.Config, o.Config):
		// envoy config, use reflect
		return false
	}
	return true
}

// Hash implements a GoString based "hash" function for ConsulUpstream; because
// this struct now contains an opaque map we cannot do much better than this.
func (u ConsulUpstream) Hash() string {
	return fmt.Sprintf("%#v", u)
}

func upstreamsEquals(a, b []ConsulUpstream) bool {
	setA := set.HashSetFrom[ConsulUpstream, string](a)
	setB := set.HashSetFrom[ConsulUpstream, string](b)
	return setA.Equal(setB)
}

// ConsulExposeConfig represents a Consul Connect expose jobspec block.
type ConsulExposeConfig struct {
	Paths []ConsulExposePath
}

type ConsulExposePath struct {
	Path          string
	Protocol      string
	LocalPathPort int
	ListenerPort  string
}

func exposePathsEqual(a, b []ConsulExposePath) bool {
	return helper.SliceSetEq(a, b)
}

// Copy the block. Returns nil if e is nil.
func (e *ConsulExposeConfig) Copy() *ConsulExposeConfig {
	if e == nil {
		return nil
	}
	paths := make([]ConsulExposePath, len(e.Paths))
	copy(paths, e.Paths)
	return &ConsulExposeConfig{
		Paths: paths,
	}
}

// Equal returns true if the structs are recursively equal.
func (e *ConsulExposeConfig) Equal(o *ConsulExposeConfig) bool {
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

	// Mesh indicates the Consul service should be a Mesh Gateway.
	Mesh *ConsulMeshConfigEntry
}

func (g *ConsulGateway) Prefix() string {
	switch {
	case g.Mesh != nil:
		return ConnectMeshPrefix
	case g.Ingress != nil:
		return ConnectIngressPrefix
	default:
		return ConnectTerminatingPrefix
	}
}

func (g *ConsulGateway) Copy() *ConsulGateway {
	if g == nil {
		return nil
	}

	return &ConsulGateway{
		Proxy:       g.Proxy.Copy(),
		Ingress:     g.Ingress.Copy(),
		Terminating: g.Terminating.Copy(),
		Mesh:        g.Mesh.Copy(),
	}
}

func (g *ConsulGateway) Equal(o *ConsulGateway) bool {
	if g == nil || o == nil {
		return g == o
	}

	if !g.Proxy.Equal(o.Proxy) {
		return false
	}

	if !g.Ingress.Equal(o.Ingress) {
		return false
	}

	if !g.Terminating.Equal(o.Terminating) {
		return false
	}

	if !g.Mesh.Equal(o.Mesh) {
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

	if err := g.Mesh.Validate(); err != nil {
		return err
	}

	// Exactly 1 of ingress/terminating/mesh must be set.
	count := 0
	if g.Ingress != nil {
		count++
	}
	if g.Terminating != nil {
		count++
	}
	if g.Mesh != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("One Consul Gateway Configuration must be set")
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

func (a *ConsulGatewayBindAddress) Equal(o *ConsulGatewayBindAddress) bool {
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
		ConnectTimeout:                  pointer.Of(*p.ConnectTimeout),
		EnvoyGatewayBindTaggedAddresses: p.EnvoyGatewayBindTaggedAddresses,
		EnvoyGatewayBindAddresses:       p.copyBindAddresses(),
		EnvoyGatewayNoDefaultBind:       p.EnvoyGatewayNoDefaultBind,
		EnvoyDNSDiscoveryType:           p.EnvoyDNSDiscoveryType,
		Config:                          maps.Clone(p.Config),
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
		if !o[listener].Equal(addr) {
			return false
		}
	}

	return true
}

func (p *ConsulGatewayProxy) Equal(o *ConsulGatewayProxy) bool {
	if p == nil || o == nil {
		return p == o
	}

	if !pointer.Eq(p.ConnectTimeout, o.ConnectTimeout) {
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

	// envoy config, use reflect
	if !reflect.DeepEqual(p.Config, o.Config) {
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

// ConsulGatewayTLSSDSConfig is used to configure the gateway's TLS listener to
// load certificates from an external Secret Discovery Service (SDS)
type ConsulGatewayTLSSDSConfig struct {

	// ClusterName specifies the name of the SDS cluster where Consul should
	// retrieve certificates.
	ClusterName string

	// CertResource specifies an SDS resource name
	CertResource string
}

func (c *ConsulGatewayTLSSDSConfig) Copy() *ConsulGatewayTLSSDSConfig {
	if c == nil {
		return nil
	}

	return &ConsulGatewayTLSSDSConfig{
		ClusterName:  c.ClusterName,
		CertResource: c.CertResource,
	}
}

func (c *ConsulGatewayTLSSDSConfig) Equal(o *ConsulGatewayTLSSDSConfig) bool {
	if c == nil || o == nil {
		return c == o
	}

	return c.ClusterName == o.ClusterName &&
		c.CertResource == o.CertResource
}

// ConsulGatewayTLSConfig is used to configure TLS for a gateway. Both
// ConsulIngressConfigEntry and ConsulIngressService use this struct. For more
// details, consult the Consul documentation:
// https://developer.hashicorp.com/consul/docs/connect/config-entries/ingress-gateway#listeners-services-tls
type ConsulGatewayTLSConfig struct {
	// Enabled indicates whether TLS is enabled for the configuration entry
	Enabled bool

	// TLSMinVersion specifies the minimum TLS version supported for gateway
	// listeners.
	TLSMinVersion string

	// TLSMaxVersion specifies the maxmimum TLS version supported for gateway
	// listeners.
	TLSMaxVersion string

	// CipherSuites specifies a list of cipher suites that gateway listeners
	// support when negotiating connections using TLS 1.2 or older.
	CipherSuites []string

	// SDS specifies parameters that configure the listener to load TLS
	// certificates from an external Secrets Discovery Service (SDS).
	SDS *ConsulGatewayTLSSDSConfig
}

func (c *ConsulGatewayTLSConfig) Copy() *ConsulGatewayTLSConfig {
	if c == nil {
		return nil
	}

	return &ConsulGatewayTLSConfig{
		Enabled:       c.Enabled,
		TLSMinVersion: c.TLSMinVersion,
		TLSMaxVersion: c.TLSMaxVersion,
		CipherSuites:  slices.Clone(c.CipherSuites),
		SDS:           c.SDS.Copy(),
	}
}

func (c *ConsulGatewayTLSConfig) Equal(o *ConsulGatewayTLSConfig) bool {
	if c == nil || o == nil {
		return c == o
	}

	return c.Enabled == o.Enabled &&
		c.TLSMinVersion == o.TLSMinVersion &&
		c.TLSMaxVersion == o.TLSMaxVersion &&
		helper.SliceSetEq(c.CipherSuites, o.CipherSuites) &&
		c.SDS.Equal(o.SDS)
}

// ConsulHTTPHeaderModifiers is a set of rules for HTTP header modification that
// should be performed by proxies as the request passes through them. It can
// operate on either request or response headers depending on the context in
// which it is used.
type ConsulHTTPHeaderModifiers struct {
	// Add is a set of name -> value pairs that should be appended to the request
	// or response (i.e. allowing duplicates if the same header already exists).
	Add map[string]string

	// Set is a set of name -> value pairs that should be added to the request or
	// response, overwriting any existing header values of the same name.
	Set map[string]string

	// Remove is the set of header names that should be stripped from the request
	// or response.
	Remove []string
}

func (h *ConsulHTTPHeaderModifiers) Copy() *ConsulHTTPHeaderModifiers {
	if h == nil {
		return nil
	}

	return &ConsulHTTPHeaderModifiers{
		Add:    maps.Clone(h.Add),
		Set:    maps.Clone(h.Set),
		Remove: slices.Clone(h.Remove),
	}
}

func (h *ConsulHTTPHeaderModifiers) Equal(o *ConsulHTTPHeaderModifiers) bool {
	if h == nil || o == nil {
		return h == o
	}

	if !maps.Equal(h.Add, o.Add) {
		return false
	}

	if !maps.Equal(h.Set, o.Set) {
		return false
	}

	if !helper.SliceSetEq(h.Remove, o.Remove) {
		return false
	}

	return true
}

// ConsulIngressService is used to configure a service fronted by the ingress gateway.
// For more details, consult the Consul documentation:
// https://developer.hashicorp.com/consul/docs/connect/config-entries/ingress-gateway
type ConsulIngressService struct {

	// Name of the service exposed through this listener.
	Name string

	// Hosts specifies one or more hosts that the listening services can receive
	// requests on.
	Hosts []string

	// TLS specifies a TLS configuration override for a specific service. If
	// unset this will fallback to the ConsulIngressConfigEntry's own TLS field.
	TLS *ConsulGatewayTLSConfig

	// RequestHeaders specifies a set of HTTP-specific header modification rules
	// applied to requests routed through the gateway
	RequestHeaders *ConsulHTTPHeaderModifiers

	// ResponseHeader specifies a set of HTTP-specific header modification rules
	// applied to responses routed through the gateway
	ResponseHeaders *ConsulHTTPHeaderModifiers

	// MaxConnections specifies the maximum number of HTTP/1.1 connections a
	// service instance is allowed to establish against the upstream
	MaxConnections *uint32

	// MaxPendingRequests specifies the maximum number of requests that are
	// allowed to queue while waiting to establish a connection
	MaxPendingRequests *uint32

	// MaxConcurrentRequests specifies the maximum number of concurrent HTTP/2
	// traffic requests that are allowed at a single point in time
	MaxConcurrentRequests *uint32
}

func (s *ConsulIngressService) Copy() *ConsulIngressService {
	if s == nil {
		return nil
	}

	ns := new(ConsulIngressService)
	*ns = *s

	ns.Hosts = slices.Clone(s.Hosts)
	ns.RequestHeaders = s.RequestHeaders.Copy()
	ns.ResponseHeaders = s.ResponseHeaders.Copy()
	ns.TLS = s.TLS.Copy()

	ns.MaxConnections = pointer.Copy(s.MaxConnections)
	ns.MaxPendingRequests = pointer.Copy(s.MaxPendingRequests)
	ns.MaxConcurrentRequests = pointer.Copy(s.MaxConcurrentRequests)

	return ns
}

func (s *ConsulIngressService) Equal(o *ConsulIngressService) bool {
	if s == nil || o == nil {
		return s == o
	}

	if s.Name != o.Name {
		return false
	}

	if !helper.SliceSetEq(s.Hosts, o.Hosts) {
		return false
	}

	if !s.TLS.Equal(o.TLS) {
		return false
	}

	if !s.RequestHeaders.Equal(o.RequestHeaders) {
		return false
	}

	if !s.ResponseHeaders.Equal(o.ResponseHeaders) {
		return false
	}

	if !pointer.Eq(s.MaxConnections, o.MaxConnections) {
		return false
	}

	if !pointer.Eq(s.MaxPendingRequests, o.MaxPendingRequests) {
		return false
	}

	if !pointer.Eq(s.MaxConcurrentRequests, o.MaxConcurrentRequests) {
		return false
	}

	return true
}

func (s *ConsulIngressService) Validate(protocol string) error {
	if s == nil {
		return nil
	}

	// pre-validate service Name and Hosts before passing along to consul:
	// https://developer.hashicorp.com/consul/docs/connect/config-entries/ingress-gateway#services

	if s.Name == "" {
		return errors.New("Consul Ingress Service requires a name")
	}

	switch protocol {
	case "tcp":
		if s.Name == "*" {
			return errors.New(`Consul Ingress Service doesn't support wildcard name for "tcp" protocol`)
		}

		if len(s.Hosts) != 0 {
			return errors.New(`Consul Ingress Service doesn't support associating hosts to a service for the "tcp" protocol`)
		}
	default:
		if s.Name == "*" && len(s.Hosts) != 0 {
			return errors.New(`Consul Ingress Service with a wildcard "*" service name can not also specify hosts`)
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

func (l *ConsulIngressListener) Equal(o *ConsulIngressListener) bool {
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

	protocols := []string{"tcp", "http", "http2", "grpc"}
	if !slices.Contains(protocols, l.Protocol) {
		return fmt.Errorf(`Consul Ingress Listener requires protocol of %s, got %q`, strings.Join(protocols, ", "), l.Protocol)
	}

	if len(l.Services) == 0 {
		return fmt.Errorf("Consul Ingress Listener requires one or more services")
	}

	for _, service := range l.Services {
		if err := service.Validate(l.Protocol); err != nil {
			return err
		}
	}

	return nil
}

func ingressServicesEqual(a, b []*ConsulIngressService) bool {
	return helper.ElementsEqual(a, b)
}

// ConsulIngressConfigEntry represents the Consul Configuration Entry type for
// an Ingress Gateway.
//
// https://www.consul.io/docs/agent/config-entries/ingress-gateway#available-fields
type ConsulIngressConfigEntry struct {

	// TLS specifies a TLS configuration for the gateway.
	TLS *ConsulGatewayTLSConfig

	// Listeners specifies a list of listeners in the mesh for the
	// gateway. Listeners are uniquely identified by their port number.
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

func (e *ConsulIngressConfigEntry) Equal(o *ConsulIngressConfigEntry) bool {
	if e == nil || o == nil {
		return e == o
	}

	if !e.TLS.Equal(o.TLS) {
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

func ingressListenersEqual(a, b []*ConsulIngressListener) bool {
	return helper.ElementsEqual(a, b)
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

func (s *ConsulLinkedService) Equal(o *ConsulLinkedService) bool {
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

func linkedServicesEqual(a, b []*ConsulLinkedService) bool {
	return helper.ElementsEqual(a, b)
}

type ConsulTerminatingConfigEntry struct {
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

func (e *ConsulTerminatingConfigEntry) Equal(o *ConsulTerminatingConfigEntry) bool {
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

// ConsulMeshConfigEntry is a stub used to represent that the gateway service
// type should be for a Mesh Gateway. Unlike Ingress and Terminating, there is no
// dedicated Consul Config Entry type for "mesh-gateway", for now. We still
// create a type for future proofing, and to keep underlying job-spec marshaling
// consistent with the other types.
type ConsulMeshConfigEntry struct {
	// nothing in here
}

func (e *ConsulMeshConfigEntry) Copy() *ConsulMeshConfigEntry {
	if e == nil {
		return nil
	}
	return new(ConsulMeshConfigEntry)
}

func (e *ConsulMeshConfigEntry) Equal(o *ConsulMeshConfigEntry) bool {
	if e == nil || o == nil {
		return e == o
	}
	return true
}

func (e *ConsulMeshConfigEntry) Validate() error {
	return nil
}
