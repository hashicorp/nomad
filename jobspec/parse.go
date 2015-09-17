package jobspec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl"
	hclobj "github.com/hashicorp/hcl/hcl"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

// Parse parses the job spec from the given io.Reader.
//
// Due to current internal limitations, the entire contents of the
// io.Reader will be copied into memory first before parsing.
func Parse(r io.Reader) (*structs.Job, error) {
	// Copy the reader into an in-memory buffer first since HCL requires it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	// Parse the buffer
	obj, err := hcl.Parse(buf.String())
	if err != nil {
		return nil, fmt.Errorf("error parsing: %s", err)
	}
	buf.Reset()

	var job structs.Job

	// Parse the job out
	jobO := obj.Get("job", false)
	if jobO == nil {
		return nil, fmt.Errorf("'job' stanza not found")
	}
	if err := parseJob(&job, jobO); err != nil {
		return nil, fmt.Errorf("error parsing 'job': %s", err)
	}

	return &job, nil
}

// ParseFile parses the given path as a job spec.
func ParseFile(path string) (*structs.Job, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return Parse(f)
}

func parseJob(result *structs.Job, obj *hclobj.Object) error {
	if obj.Len() > 1 {
		return fmt.Errorf("only one 'job' block allowed")
	}

	// Get our job object
	obj = obj.Elem(true)[0]

	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, obj); err != nil {
		return err
	}
	delete(m, "constraint")
	delete(m, "meta")

	// Set the ID and name to the object key
	result.ID = obj.Key
	result.Name = obj.Key

	// Defaults
	result.Priority = 50
	result.Region = "global"
	result.Type = "service"

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Parse constraints
	if o := obj.Get("constraint", false); o != nil {
		if err := parseConstraints(&result.Constraints, o); err != nil {
			return err
		}
	}

	// Parse out meta fields. These are in HCL as a list so we need
	// to iterate over them and merge them.
	if metaO := obj.Get("meta", false); metaO != nil {
		for _, o := range metaO.Elem(false) {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &result.Meta); err != nil {
				return err
			}
		}
	}

	// If we have tasks outside, do those
	if o := obj.Get("task", false); o != nil {
		var tasks []*structs.Task
		if err := parseTasks(&tasks, o); err != nil {
			return err
		}

		result.TaskGroups = make([]*structs.TaskGroup, len(tasks), len(tasks)*2)
		for i, t := range tasks {
			result.TaskGroups[i] = &structs.TaskGroup{
				Name:  t.Name,
				Count: 1,
				Tasks: []*structs.Task{t},
			}
		}
	}

	// Parse the task groups
	if o := obj.Get("group", false); o != nil {
		if err := parseGroups(result, o); err != nil {
			return fmt.Errorf("error parsing 'group': %s", err)
		}
	}

	return nil
}

func parseGroups(result *structs.Job, obj *hclobj.Object) error {
	// Get all the maps of keys to the actual object
	objects := make(map[string]*hclobj.Object)
	for _, o1 := range obj.Elem(false) {
		for _, o2 := range o1.Elem(true) {
			if _, ok := objects[o2.Key]; ok {
				return fmt.Errorf(
					"group '%s' defined more than once",
					o2.Key)
			}

			objects[o2.Key] = o2
		}
	}

	if len(objects) == 0 {
		return nil
	}

	// Go through each object and turn it into an actual result.
	collection := make([]*structs.TaskGroup, 0, len(objects))
	for n, o := range objects {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
			return err
		}
		delete(m, "constraint")
		delete(m, "meta")
		delete(m, "task")

		// Build the group with the basic decode
		var g structs.TaskGroup
		g.Name = n
		if err := mapstructure.WeakDecode(m, &g); err != nil {
			return err
		}

		// Parse constraints
		if o := o.Get("constraint", false); o != nil {
			if err := parseConstraints(&g.Constraints, o); err != nil {
				return err
			}
		}

		// Parse out meta fields. These are in HCL as a list so we need
		// to iterate over them and merge them.
		if metaO := o.Get("meta", false); metaO != nil {
			for _, o := range metaO.Elem(false) {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &g.Meta); err != nil {
					return err
				}
			}
		}

		// Parse tasks
		if o := o.Get("task", false); o != nil {
			if err := parseTasks(&g.Tasks, o); err != nil {
				return err
			}
		}

		collection = append(collection, &g)
	}

	result.TaskGroups = append(result.TaskGroups, collection...)
	return nil
}

func parseConstraints(result *[]*structs.Constraint, obj *hclobj.Object) error {
	for _, o := range obj.Elem(false) {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
			return err
		}
		m["LTarget"] = m["attribute"]
		m["RTarget"] = m["value"]
		m["Operand"] = m["operator"]

		// Build the constraint
		var c structs.Constraint
		if err := mapstructure.WeakDecode(m, &c); err != nil {
			return err
		}
		if c.Operand == "" {
			c.Operand = "="
		}

		*result = append(*result, &c)
	}

	return nil
}

func parseTasks(result *[]*structs.Task, obj *hclobj.Object) error {
	// Get all the maps of keys to the actual object
	objects := make([]*hclobj.Object, 0, 5)
	set := make(map[string]struct{})
	for _, o1 := range obj.Elem(false) {
		for _, o2 := range o1.Elem(true) {
			if _, ok := set[o2.Key]; ok {
				return fmt.Errorf(
					"group '%s' defined more than once",
					o2.Key)
			}

			objects = append(objects, o2)
			set[o2.Key] = struct{}{}
		}
	}

	if len(objects) == 0 {
		return nil
	}

	for _, o := range objects {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
			return err
		}
		delete(m, "config")
		delete(m, "constraint")
		delete(m, "meta")
		delete(m, "resources")

		// Build the task
		var t structs.Task
		t.Name = o.Key
		if err := mapstructure.WeakDecode(m, &t); err != nil {
			return err
		}

		// If we have config, then parse that
		if o := o.Get("config", false); o != nil {
			for _, o := range o.Elem(false) {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &t.Config); err != nil {
					return err
				}
			}
		}

		// Parse constraints
		if o := o.Get("constraint", false); o != nil {
			if err := parseConstraints(&t.Constraints, o); err != nil {
				return err
			}
		}

		// Parse out meta fields. These are in HCL as a list so we need
		// to iterate over them and merge them.
		if metaO := o.Get("meta", false); metaO != nil {
			for _, o := range metaO.Elem(false) {
				var m map[string]interface{}
				if err := hcl.DecodeObject(&m, o); err != nil {
					return err
				}
				if err := mapstructure.WeakDecode(m, &t.Meta); err != nil {
					return err
				}
			}
		}

		// If we have resources, then parse that
		if o := o.Get("resources", false); o != nil {
			var r structs.Resources
			if err := parseResources(&r, o); err != nil {
				return fmt.Errorf("task '%s': %s", t.Name, err)
			}

			t.Resources = &r
		}

		*result = append(*result, &t)
	}

	return nil
}

func parseResources(result *structs.Resources, obj *hclobj.Object) error {
	if obj.Len() > 1 {
		return fmt.Errorf("only one 'resource' block allowed per task")
	}

	for _, o := range obj.Elem(false) {
		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o); err != nil {
			return err
		}
		delete(m, "network")

		if err := mapstructure.WeakDecode(m, result); err != nil {
			return err
		}

		// Parse the network resources
		if o := o.Get("network", false); o != nil {
			if o.Len() > 1 {
				return fmt.Errorf("only one 'network' resource allowed")
			}

			var r structs.NetworkResource
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &r); err != nil {
				return err
			}

			result.Networks = []*structs.NetworkResource{&r}
		}

	}

	return nil
}
