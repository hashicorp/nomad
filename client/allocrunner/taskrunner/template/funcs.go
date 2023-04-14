package template

import (
	"fmt"
	"text/template"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	hargs "github.com/hashicorp/nomad/helper/args"
	"golang.org/x/exp/slices"
)

type NomadFuncMapConfig struct {
	sandboxEnabled   bool
	functionDenyList []string
	te               *taskenv.TaskEnv
	uid              *int
	gid              *int
}

func NewNomadEnvFuncMap(c NomadFuncMapConfig) template.FuncMap {
	fm := template.FuncMap{
		"env":             Env(c.te),
		"sprig_env":       Env(c.te),
		"mustEnv":         MustEnv(c.te),
		"sprig_expandenv": ExpandEnv(c.te),
		"node_attrs":      NodeAttrs(c.te),
		"node_attr":       NodeAttr(c.te),
	}
	errSuffix := "has been disabled by an administrator"
	if slices.Equal(c.functionDenyList, config.DefaultTemplateFunctionDenylist) {
		errSuffix = "is disabled by default."
	}
	for _, fName := range c.functionDenyList {
		fm[fName] = func(_ ...interface{}) (interface{}, error) {
			return nil, fmt.Errorf("the %q function %s", fName, errSuffix)
		}
	}
	return fm
}

// Env opaques consul-template's provided env function with one that only
// can consult the taskenv's EnvMap
func Env(te *taskenv.TaskEnv) func(v string) string {
	return func(v string) string {
		te := te
		return fromMap("environment variable", te.All(), v)
	}
}

// MustEnv opaques consul-template's provided mustEnv function with one that
// only can consult the taskenv's EnvMap
func MustEnv(te *taskenv.TaskEnv) func(v string) (string, error) {
	return func(v string) (string, error) {
		return mustFromMap("environment variable", te.All(), v)
	}
}

func ExpandEnv(te *taskenv.TaskEnv) func(v string) string {
	return func(v string) string {
		return hargs.ReplaceEnv(v, te.EnvMap, te.NodeAttrs)
	}
}

// envOrDefault returns the value from the task environment or the provided
// default when the value is not found.
func EnvOrDefault(te *taskenv.TaskEnv) func(v, d string) string {
	return func(v, d string) string {
		out, err := mustFromMap("environment variable", te.All(), v)
		if err != nil {
			out = d
		}
		return out
	}
}

// NodeAttrs provides the NodeAttrs map to CT as a rangeable map
func NodeAttrs(te *taskenv.TaskEnv) func() map[string]string {
	return func() map[string]string {
		return te.NodeAttrs
	}
}

// NodeAttr returns the value of a specific node attribute and "" if it's
// not available
func NodeAttr(te *taskenv.TaskEnv) func(v string) string {
	return func(v string) string {
		return fromMap("node attribute", te.NodeAttrs, v)
	}
}

// MustNodeAttr returns the value of a specific node attribute and "" if it's
// not available
func MustNodeAttr(te *taskenv.TaskEnv) func(v string) (string, error) {
	return func(v string) (string, error) {
		return mustFromMap("node attribute", te.NodeAttrs, v)
	}
}

// fromMap queries the provided map for a given key and returns it if found
// or "" when not found
func fromMap(on string, s map[string]string, k string) string {
	out, _ := mustFromMap(on, s, k)
	return out
}

// mustFromMap queries the provided map for a given key and returns it if found
// or an error when the value is not found
func mustFromMap(on string, s map[string]string, k string) (string, error) {
	var v string
	var ok bool
	if v, ok = s[k]; !ok {
		return "", fmt.Errorf("%s not found", on)
	}
	return v, nil
}
