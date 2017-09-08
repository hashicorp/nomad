package sentinel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/sentinel/cmd/format"
	"github.com/hashicorp/sentinel/imports/static"
	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/runtime/eval"
	"github.com/hashicorp/sentinel/runtime/trace"
)

// IsUndefined returns true if the resulting error value from Eval is
// due to undefined behavior.
func IsUndefined(err error) bool {
	_, ok := err.(*eval.UndefError)
	return ok
}

// IsRuntime returns true if the resulting error value from Eval is
// due to a runtie error.
func IsRuntime(err error) bool {
	_, ok := err.(*eval.EvalError)
	return ok
}

// EvalOpts are optional values that can be set for evaluation. None
// of these are required. They can alter the default behavior of an
// evaluation.
type EvalOpts struct {
	// Data is the a set of data to inject into the global scope of
	// the policies. This map must not be modified during execution.
	Data map[string]interface{}

	// Override for soft mandatory policies. The host system may decide
	// the mechanism for how overrides are set.
	Override bool

	// EvalAll will force all policies to evaluate even if Sentinel
	// detects an ability to short-circuit the evaluation. This will
	// increase execution time for failure scenarios.
	EvalAll bool

	// Trace, if true, will trace all evaluated policies. This will
	// mean the Trace field on EvalPolicyResult will be populated.
	Trace bool
}

// EvalResult is the result of calling Eval.
type EvalResult struct {
	// Result is the simple pass/fail of the set of policies. If you
	// care about nothing other than knowing if Sentinel passed, use this.
	Result bool `json:"result"`

	// Error is non-nil if there is an error to report in addition to
	// a failure. If this is non-nil, Result will always be false.
	Error error `json:"error"`

	// CanOverride is true if the Result would be true if there was
	// an override set. In this case, you don't need to re-run policies
	// if you can get an override. Note that if policies are time sensitive
	// you may still want to re-run them.
	//
	// EvalAll must be set for this to exist. EvalAll is required because
	// we can't know whether the policies would've passed unless we
	// evaluated all of them.
	//
	// This field is meaningless if Result is already true.
	CanOverride bool `json:"can_override,omitempty"`

	// Policies is the list of policies that were executed and evaluation
	// information about each. Note that this may not be all the policies
	// given as input since Sentinel will short-circuit evaluation unless
	// told otherwise.
	//
	// These are not sorted in any particular order.
	Policies []*EvalPolicyResult `json:"policies"`
}

// EvalPolicyResult is the result of an evaluation of a single policy.
type EvalPolicyResult struct {
	// The policy this is referencing. Note that if this has been
	// unlocked already, then it is not safe to read. This value must
	// be read prior to the policy being unlocked since there is no way
	// to reacquire a policy lock.
	Policy *Policy `json:"policy"`

	// The result for this policy.
	Result bool `json:"result"`

	// AllowedFailure is true if this is an advisory or soft mandatory
	// policy that failed but was allowed to. For advisory policies, they
	// are always allowed to fail. For soft mandatory, this means that
	// the override was set. Host systems may use this information to log
	// allows failures.
	AllowedFailure bool `json:"allowed_failure"`

	// Error will contain the error from evaluation for this specific
	// Policy. This differs from EvalResult.Error because that field
	// may contain the combined errors from multiple policies. If this
	// is non-nil, it is guaranteed that EvalResult.Error is non-nil.
	Error error `json:"error"`

	// Trace is the evaluation trace for this policy. This will only
	// be set if tracing is enabled via EvalOpts (disabled by default).
	Trace *trace.Trace `json:"trace"`
}

// Eval evaluates a set of policies and returns the result of the policy
// execution along with any error that may have occurred.
//
// The policies must be safe for reading, which typically means that a
// read lock is held. Sentinel will do this automatically if the policy
// was retrieved via the Policy method.
//
// Sentinel may or may not execute all policies and the order is also
// not guaranteed. Depending on the configuration, policies may be executed
// in parallel.
//
// The optional EvalOpts argument can be set to inject implicit globals,
// enable tracing, and more. See EvalOpts for documentation.
func (s *Sentinel) Eval(ps []*Policy, opts *EvalOpts) *EvalResult {
	// Build our result
	result := &EvalResult{
		Policies: make([]*EvalPolicyResult, 0, len(ps)),
	}

	// Build the scope which can be shared by all
	scope := object.NewScope(eval.Universe)
	if opts != nil {
		for k, v := range opts.Data {
			obj, err := static.NewObject(v)
			if err != nil {
				result.Error = fmt.Errorf(
					"couldn't convert data %q to Sentinel object: %s",
					k, err)
				return result
			}

			scope.Objects[k] = obj
		}
	}

	// At the start, all policies are passing
	result.Result = true
	hardFail := false

	// Currently we just do a naive sequential execution with zero
	// performance improvements. We'll improve this later.
	for _, p := range ps {
		impt := &sentinelImporter{
			Sentinel: s,
			Policy:   p,
		}

		var traceOut *trace.Trace
		if opts != nil && opts.Trace {
			traceOut = &trace.Trace{}
		}

		// Evaluate
		evalResult, err := eval.Eval(&eval.EvalOpts{
			Compiled: p.compiled,
			Scope:    scope,
			Importer: impt,
			Timeout:  s.evalTimeout,
			Trace:    traceOut,
		})

		// Clean up our imports
		impt.Close()

		// Build the result
		policyResult := &EvalPolicyResult{
			Policy: p,
			Result: evalResult,
			Error:  err,
			Trace:  traceOut,
		}
		result.Policies = append(result.Policies, policyResult)

		// If there is an error and we passed... we need to return right
		// away. That is very strange. We explicitly set the policy to false
		// since that is dangerous. We don't respect enforcement levels in
		// this case because this represents a situation that shouldn't arise.
		if evalResult && err != nil {
			result.Result = false
			result.Error = fmt.Errorf(
				"Sentinel resulted in true but raised an error: %s", err)
			return result
		}

		// If we failed a policy, we need to handle that based on the
		// enforcement level.
		if !evalResult {
			switch lvl := p.Level(); lvl {
			case Advisory:
				// Advisory policy means that it is allowed to fail. However,
				// note that we allowed it to fail.
				policyResult.AllowedFailure = true

			case SoftMandatory:
				// If we have the override set, we allow soft mandatory
				// policies to fail.
				if opts.Override {
					policyResult.AllowedFailure = true
					break
				}

				// If override was set, we would've passed
				if !hardFail && opts != nil && opts.EvalAll {
					result.CanOverride = true
				}

				fallthrough

			case HardMandatory:
				fallthrough

			default:
				if lvl == HardMandatory {
					// Note that we've had a hard failure
					hardFail = true

					// Mark CanOverride false because no override would save us
					result.CanOverride = false
				}

				// Default to hard mandatory
				result.Result = false

				// If there is an error, set that
				if err != nil {
					result.Error = err
				}

				if opts != nil && !opts.EvalAll {
					return result
				}
			}
		}

	}

	return result
}

// json.Marshaller
func (r *EvalResult) MarshalJSON() ([]byte, error) {
	var errString *string
	if r.Error != nil {
		s := r.Error.Error()
		errString = &s
	}

	return json.Marshal(map[string]interface{}{
		"result":       r.Result,
		"error":        errString,
		"can_override": r.CanOverride,
		"policies":     r.Policies,
	})
}

// String returns a human-friendly format for the result. This format can
// easily be used for output to an API, CLI, file, etc.
//
// The result of this is optimized for a fixed-width screen. Line width
// is limited to 80 characters where possible. This may overflow sometimes
// due to the size of expressions in traces.
//
// This String method assumes you're still holding a lock on the Policy
// structures within the Policies slice, so call String() before you unlock
// any policies.
func (r *EvalResult) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Sentinel Result: %v\n\n", r.Result))

	// Output more details about what the result means
	switch {
	case r.Result:
		buf.WriteString(strings.TrimSpace(evalResultTrue))

	case IsUndefined(r.Error):
		buf.WriteString(strings.TrimSpace(evalResultUndefined))

	case IsRuntime(r.Error):
		buf.WriteString(strings.TrimSpace(evalResultError))

	case !r.Result:
		buf.WriteString(strings.TrimSpace(evalResultFalse))
	}
	buf.WriteString("\n\n")

	if r.Error != nil {
		buf.WriteString(fmt.Sprintf("Error message: %s\n\n", r.Error))
	}

	if len(r.Policies) > 0 {
		buf.WriteString(fmt.Sprintf("%d policies evaluated.\n\n", len(r.Policies)))

		// Print failed policies first
		idx := 1
		for _, p := range r.Policies {
			if !p.Result {
				buf.WriteString(fmt.Sprintf("## Policy %d: %s\n\n", idx, p.Policy.Name()))
				p.stringBuf(&buf)
				idx++
			}
		}

		// Good policies
		for _, p := range r.Policies {
			if p.Result {
				buf.WriteString(fmt.Sprintf("## Policy %d: %s\n\n", idx, p.Policy.Name()))
				p.stringBuf(&buf)
				idx++
			}
		}
	}

	return buf.String()
}

func (r *EvalPolicyResult) String() string {
	var buf bytes.Buffer
	r.stringBuf(&buf)
	return buf.String()
}

func (r *EvalPolicyResult) stringBuf(buf *bytes.Buffer) {
	allowedFailure := ""
	if r.AllowedFailure {
		allowedFailure = " (allowed failure based on level)"
	}

	buf.WriteString(fmt.Sprintf("Result: %v%s\n\n", r.Result, allowedFailure))

	// Write description
	if d := r.Policy.Doc(); d != "" {
		buf.WriteString(fmt.Sprintf("Description: %s\n\n", d))
	}

	if r.Error != nil {
		buf.WriteString(fmt.Sprintf("Error message: %s\n\n", r.Error))
	}

	if r.Trace != nil {
		if r.Trace.Print.Len() > 0 {
			buf.WriteString(fmt.Sprintf(
				"Print messages:\n\n%s\n", r.Trace.Print.String()))
		}

		ruleFmt := &format.RuleTrace{
			FileSet: r.Policy.FileSet(),
			Rules:   r.Trace.Rules,
		}

		buf.WriteString(ruleFmt.String())
	}
}

// json.Marshaller
func (r *EvalPolicyResult) MarshalJSON() ([]byte, error) {
	var errString *string
	if r.Error != nil {
		s := r.Error.Error()
		errString = &s
	}

	return json.Marshal(map[string]interface{}{
		"policy":          r.Policy,
		"result":          r.Result,
		"allowed_failure": r.AllowedFailure,
		"error":           errString,
		"trace":           r.Trace,
	})
}

const evalResultTrue = `
This result means that Sentinel policies returned true and the protected
behavior is allowed by Sentinel policies.
`

const evalResultUndefined = `
Sentinel evaluated to false because one or more policies below resulted
in an "undefined" value being the result of the main rule. This is typically
indicative of a logical bug within a policy. Please see the details of the
policies executed below to find the result with the undefined value.
`

const evalResultError = `
Sentinel evaluated to false because of a runtime error. This error message
will be shown below. Please see the details of the policies executed below
to find the exact policy that experienced the runtime error.
`

const evalResultFalse = `
Sentinel evaluated to false because one or more Sentinel policies evaluated
to false. This false was not due to an undefined value or runtime error.
`
