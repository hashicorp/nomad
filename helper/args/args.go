package args

import "regexp"

var (
	envRe = regexp.MustCompile(`\$({[a-zA-Z0-9_]+}|[a-zA-Z0-9_]+)`)
)

// ParseAndReplace takes the user supplied args and a map of environment
// variables. It replaces any instance of an environment variable in the args
// with the actual value.
func ParseAndReplace(args []string, env map[string]string) []string {
	replaced := make([]string, len(args))
	for i, arg := range args {
		replaced[i] = ReplaceEnv(arg, env)
	}

	return replaced
}

// ReplaceEnv takes an arg and replaces all occurences of environment variables.
// If the variable is found in the passed map it is replaced, otherwise the
// original string is returned.
func ReplaceEnv(arg string, env map[string]string) string {
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
