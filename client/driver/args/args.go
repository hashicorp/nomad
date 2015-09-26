package args

import (
	"fmt"
	"regexp"

	"github.com/mattn/go-shellwords"
)

var (
	envRe = regexp.MustCompile(`\$({[a-zA-Z0-9_]+}|[a-zA-Z0-9_]+)`)
)

// ParseAndReplace takes the user supplied args and a map of environment
// variables. It replaces any instance of an environment variable in the args
// with the actual value and does correct splitting of the arg list.
func ParseAndReplace(args string, env map[string]string) ([]string, error) {
	// Set up parser.
	p := shellwords.NewParser()
	p.ParseEnv = false
	p.ParseBacktick = false

	parsed, err := p.Parse(args)
	if err != nil {
		return nil, fmt.Errorf("Couldn't parse args %v: %v", args, err)
	}

	replaced := make([]string, len(parsed))
	for i, arg := range parsed {
		replaced[i] = replaceEnv(arg, env)
	}

	return replaced, nil
}

// replaceEnv takes an arg and replaces all occurences of environment variables.
// If the variable is found in the passed map it is replaced, otherwise the
// original string is returned.
func replaceEnv(arg string, env map[string]string) string {
	return envRe.ReplaceAllStringFunc(arg, func(arg string) string {
		stripped := arg[1:]
		if stripped[0] == '{' {
			stripped = stripped[1 : len(stripped)-1]
		}

		if value, ok := env[stripped]; ok {
			return value
		}

		return arg
	})
}
