package datalog

import (
	"fmt"
	"strings"

	"github.com/kevinawalsh/datalog/dlengine"
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
	d.Retract(name)
	d.input[name] = input
	d.engine.Batch(name, input)
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

func (d *DB) Retract(name string) {
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
