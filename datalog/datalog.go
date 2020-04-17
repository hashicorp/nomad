package datalog

import (
	"fmt"
	"strings"

	"github.com/kevinawalsh/datalog/dlengine"
	"github.com/kr/pretty"
)

type DB struct {
	engine *dlengine.Engine
	input  map[string]string
}

func NewDB() *DB {
	return &DB{
		engine: dlengine.NewEngine(),
		input:  map[string]string{},
	}
}

func Name(names ...string) string {
	return strings.Join(names, "-")
}

func JobName(namespace, jobID string) string {
	return Name("job", namespace, jobID)
}

func GroupName(jobName, groupName string) string {
	return Name(jobName, groupName)
}

func TaskName(groupName, taskName string) string {
	return Name(groupName, taskName)
}

func (d *DB) Assert(name, input string) {
	d.input[name] = input
	_, _, err := d.engine.Batch(name, input)
	if err != nil {
		pretty.Log("ASSERT ERR", err)
	}
}

func isAssertion(line string) bool {
	l := len(line)
	return l > 0 && line[l-1] == '.'
}

func retraction(assertion string) string {
	l := len(assertion)
	if l == 0 {
		return ""
	}
	return assertion[:l-1] + "~"
}

func isQuery(line string) bool {
	l := len(line)
	return l > 0 && line[l-1] == '?'
}

func (d *DB) Allow(input string) bool {
	var negate bool
	for _, l := range lines(input) {
		if !isQuery(l) {
			continue
		}

		negate = false
		if l[0] == '~' {
			l = l[1:]
			negate = true
		}

		if len(d.Query(l)) == 0 {
			if !negate {
				return false
			}
		} else if negate {
			return false
		}
	}

	return true
}

func (d *DB) Query(input string) []string {
	var output []string
	answers, err := d.engine.Query(input)

	if err != nil {
		return []string{fmt.Sprintf("error: %s", err.Error())}
	}

	for _, a := range answers {
		output = append(output, a.String())
	}
	return output
}

func (d *DB) Retract(input string) {
	for _, l := range lines(input) {
		if isAssertion(l) {
			err := d.engine.Retract(retraction(l))
			if err != nil {
				pretty.Log("ERROR", err)
			}
		}
	}
}

// WithTempRules applys all the assertions in the input to the database, executes the thunk,
// and then retracts the assertions in input
func (d *DB) WithTempRules(input string, thunk func()) {
	for _, l := range lines(input) {
		if isAssertion(l) {
			d.engine.Assert(l)
		}
	}

	thunk()

	d.Retract(input)
}

func lines(input string) []string {
	var output, multi []string

	raw := strings.Split(input, "\n")
	for i := 0; i < len(raw); i++ {
		l := raw[i]
		l = strings.TrimSpace(l)
		if len(l) == 0 {
			continue
		}

		if l[len(l)-1] == []byte("\\")[0] {
			multi = append(multi, l)
		} else {
			if len(multi) > 0 {
				output = append(output, strings.Join(multi, ""))
				multi = []string{}
			} else {
				output = append(output, l)
			}
		}
	}

	return output
}
