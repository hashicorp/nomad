// Package trace contains structures to represent execution tracing.
package trace

import (
	"bytes"
	"encoding/json"
)

// Trace represents the result of a tracing operation. Tracing can be
// enabled by evaluating with Trace set to true.
type Trace struct {
	Desc   string           // Description of the policy, if non-empty
	Result bool             // result of this policy, always pass or fail
	Err    error            // error, if any, while executing this policy
	Print  bytes.Buffer     // print() data, newline separated
	Rules  map[string]*Rule // rules by name
}

func (t *Trace) MarshalJSON() ([]byte, error) {
	var errString *string
	if t.Err != nil {
		s := t.Err.Error()
		errString = &s
	}

	data := map[string]interface{}{
		"description": t.Desc,
		"result":      t.Result,
		"error":       errString,
		"print":       t.Print.String(),
		"rules":       t.Rules,
	}

	return json.Marshal(data)
}
