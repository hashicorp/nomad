// Package trace contains structures to represent execution tracing.
package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
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

// String outputs the trace in a text-only human-friendly format.
func (t *Trace) String() string {
	var buf bytes.Buffer

	// Header information
	buf.WriteString(fmt.Sprintf("Result: %v\n", t.Result))
	if t.Err != nil {
		buf.WriteString(fmt.Sprintf("Error: %v\n", t.Result))
	}
	buf.WriteRune('\n')

	// Print the description
	desc := "<none>"
	if t.Desc != "" {
		desc = t.Desc
	}
	buf.WriteString(fmt.Sprintf("Description: %s\n\n", desc))

	// Print buffer
	if t.Print.Len() > 0 {
		buf.WriteString("print() output:\n\n")
		buf.WriteString(t.Print.String())
		buf.WriteString("\n\n")
	}

	// Do other rules, first sort by their name
	keys := make([]string, 0, len(t.Rules))
	for k, _ := range t.Rules {
		if k != "main" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// Do the main rule first if we have it
	if r, ok := t.Rules["main"]; ok {
		buf.WriteString(r.String())
	}

	// Output each rule
	for _, k := range keys {
		buf.WriteRune('\n')
		buf.WriteString(t.Rules[k].String())
	}

	return buf.String()
}
