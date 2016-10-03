package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/burntsushi/toml"
	dep "github.com/hashicorp/consul-template/dependency"
	yaml "gopkg.in/yaml.v2"
)

// now is function that represents the current time in UTC. This is here
// primarily for the tests to override times.
var now = func() time.Time { return time.Now().UTC() }

// datacentersFunc returns or accumulates datacenter dependencies.
func datacentersFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(...string) ([]string, error) {
	return func(s ...string) ([]string, error) {
		result := []string{}

		d, err := dep.ParseDatacenters(s...)
		if err != nil {
			return result, err
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			return value.([]string), nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// fileFunc returns or accumulates file dependencies.
func fileFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(string) (string, error) {
	return func(s string) (string, error) {
		if len(s) == 0 {
			return "", nil
		}

		d, err := dep.ParseFile(s)
		if err != nil {
			return "", err
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			if value == nil {
				return "", nil
			}
			return value.(string), nil
		}

		addDependency(missing, d)

		return "", nil
	}
}

// keyFunc returns or accumulates key dependencies.
func keyFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(string) (string, error) {
	return func(s string) (string, error) {
		if len(s) == 0 {
			return "", nil
		}

		d, err := dep.ParseStoreKey(s)
		if err != nil {
			return "", err
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			if value == nil {
				return "", nil
			}
			return value.(string), nil
		}

		addDependency(missing, d)

		return "", nil
	}
}

// keyWithDefaultFunc returns or accumulates key dependencies that have a
// default value.
func keyWithDefaultFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(string, string) (string, error) {
	return func(s, def string) (string, error) {
		if len(s) == 0 {
			return def, nil
		}

		d, err := dep.ParseStoreKey(s)
		if err != nil {
			return "", err
		}
		d.SetDefault(def)

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			if value == nil {
				return def, nil
			}
			return value.(string), nil
		}

		addDependency(missing, d)

		return def, nil
	}
}

// lsFunc returns or accumulates keyPrefix dependencies.
func lsFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(string) ([]*dep.KeyPair, error) {
	return func(s string) ([]*dep.KeyPair, error) {
		result := []*dep.KeyPair{}

		if len(s) == 0 {
			return result, nil
		}

		d, err := dep.ParseStoreKeyPrefix(s)
		if err != nil {
			return result, err
		}

		addDependency(used, d)

		// Only return non-empty top-level keys
		if value, ok := brain.Recall(d); ok {
			for _, pair := range value.([]*dep.KeyPair) {
				if pair.Key != "" && !strings.Contains(pair.Key, "/") {
					result = append(result, pair)
				}
			}
			return result, nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// nodeFunc returns or accumulates catalog node dependency.
func nodeFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(...string) (*dep.NodeDetail, error) {
	return func(s ...string) (*dep.NodeDetail, error) {

		d, err := dep.ParseCatalogNode(s...)
		if err != nil {
			return nil, err
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			return value.(*dep.NodeDetail), nil
		}

		addDependency(missing, d)

		return nil, nil
	}
}

// nodesFunc returns or accumulates catalog node dependencies.
func nodesFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(...string) ([]*dep.Node, error) {
	return func(s ...string) ([]*dep.Node, error) {
		result := []*dep.Node{}

		d, err := dep.ParseCatalogNodes(s...)
		if err != nil {
			return nil, err
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			return value.([]*dep.Node), nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// secretFunc returns or accumulates secret dependencies from Vault.
func secretFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(...string) (*dep.Secret, error) {
	return func(s ...string) (*dep.Secret, error) {
		result := &dep.Secret{}

		if len(s) == 0 {
			return result, nil
		}

		d, err := dep.ParseVaultSecret(s...)
		if err != nil {
			return result, nil
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			result = value.(*dep.Secret)
			return result, nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// secretsFunc returns or accumulates a list of secret dependencies from Vault.
func secretsFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(string) ([]string, error) {
	return func(s string) ([]string, error) {
		result := []string{}

		if len(s) == 0 {
			return result, nil
		}

		d, err := dep.ParseVaultSecrets(s)
		if err != nil {
			return result, nil
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			result = value.([]string)
			return result, nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// serviceFunc returns or accumulates health service dependencies.
func serviceFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(...string) ([]*dep.HealthService, error) {
	return func(s ...string) ([]*dep.HealthService, error) {
		result := []*dep.HealthService{}

		if len(s) == 0 || s[0] == "" {
			return result, nil
		}

		d, err := dep.ParseHealthServices(s...)
		if err != nil {
			return nil, err
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			return value.([]*dep.HealthService), nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// servicesFunc returns or accumulates catalog services dependencies.
func servicesFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(...string) ([]*dep.CatalogService, error) {
	return func(s ...string) ([]*dep.CatalogService, error) {
		result := []*dep.CatalogService{}

		d, err := dep.ParseCatalogServices(s...)
		if err != nil {
			return nil, err
		}

		addDependency(used, d)

		if value, ok := brain.Recall(d); ok {
			return value.([]*dep.CatalogService), nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// treeFunc returns or accumulates keyPrefix dependencies.
func treeFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(string) ([]*dep.KeyPair, error) {
	return func(s string) ([]*dep.KeyPair, error) {
		result := []*dep.KeyPair{}

		if len(s) == 0 {
			return result, nil
		}

		d, err := dep.ParseStoreKeyPrefix(s)
		if err != nil {
			return result, err
		}

		addDependency(used, d)

		// Only return non-empty top-level keys
		if value, ok := brain.Recall(d); ok {
			for _, pair := range value.([]*dep.KeyPair) {
				parts := strings.Split(pair.Key, "/")
				if parts[len(parts)-1] != "" {
					result = append(result, pair)
				}
			}
			return result, nil
		}

		addDependency(missing, d)

		return result, nil
	}
}

// vaultFunc is deprecated. Use secretFunc instead.
func vaultFunc(brain *Brain,
	used, missing map[string]dep.Dependency) func(string) (*dep.Secret, error) {
	return func(s string) (*dep.Secret, error) {
		log.Printf("[WARN] the `vault' template function has been deprecated. " +
			"Please use `secret` instead!")
		return secretFunc(brain, used, missing)(s)
	}
}

// byKey accepts a slice of KV pairs and returns a map of the top-level
// key to all its subkeys. For example:
//
//		elasticsearch/a //=> "1"
//		elasticsearch/b //=> "2"
//		redis/a/b //=> "3"
//
// Passing the result from Consul through byTag would yield:
//
// 		map[string]map[string]string{
//	  	"elasticsearch": &dep.KeyPair{"a": "1"}, &dep.KeyPair{"b": "2"},
//			"redis": &dep.KeyPair{"a/b": "3"}
//		}
//
// Note that the top-most key is stripped from the Key value. Keys that have no
// prefix after stripping are removed from the list.
func byKey(pairs []*dep.KeyPair) (map[string]map[string]*dep.KeyPair, error) {
	m := make(map[string]map[string]*dep.KeyPair)
	for _, pair := range pairs {
		parts := strings.Split(pair.Key, "/")
		top := parts[0]
		key := strings.Join(parts[1:], "/")

		if key == "" {
			// Do not add a key if it has no prefix after stripping.
			continue
		}

		if _, ok := m[top]; !ok {
			m[top] = make(map[string]*dep.KeyPair)
		}

		newPair := *pair
		newPair.Key = key
		m[top][key] = &newPair
	}

	return m, nil
}

// byTag is a template func that takes the provided services and
// produces a map based on Service tags.
//
// The map key is a string representing the service tag. The map value is a
// slice of Services which have the tag assigned.
func byTag(in interface{}) (map[string][]interface{}, error) {
	m := make(map[string][]interface{})

	switch typed := in.(type) {
	case nil:
	case []*dep.CatalogService:
		for _, s := range typed {
			for _, t := range s.Tags {
				m[t] = append(m[t], s)
			}
		}
	case []*dep.HealthService:
		for _, s := range typed {
			for _, t := range s.Tags {
				m[t] = append(m[t], s)
			}
		}
	default:
		return nil, fmt.Errorf("byTag: wrong argument type %T", in)
	}

	return m, nil
}

// contains is a function that have reverse arguments of "in" and is designed to
// be used as a pipe instead of a function:
//
// 		{{ l | contains "thing" }}
//
func contains(v, l interface{}) (bool, error) {
	return in(l, v)
}

// env returns the value of the environment variable set
func env(s string) (string, error) {
	return os.Getenv(s), nil
}

// explode is used to expand a list of keypairs into a deeply-nested hash.
func explode(pairs []*dep.KeyPair) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, pair := range pairs {
		if err := explodeHelper(m, pair.Key, pair.Value, pair.Key); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// explodeHelper is a recursive helper for explode.
func explodeHelper(m map[string]interface{}, k, v, p string) error {
	if strings.Contains(k, "/") {
		parts := strings.Split(k, "/")
		top := parts[0]
		key := strings.Join(parts[1:], "/")

		if _, ok := m[top]; !ok {
			m[top] = make(map[string]interface{})
		}
		nest, ok := m[top].(map[string]interface{})
		if !ok {
			return fmt.Errorf("not a map: %q: %q already has value %q", p, top, m[top])
		}
		return explodeHelper(nest, key, v, k)
	} else {
		if k != "" {
			m[k] = v
		}
	}

	return nil
}

// in seaches for a given value in a given interface.
func in(l, v interface{}) (bool, error) {
	lv := reflect.ValueOf(l)
	vv := reflect.ValueOf(v)

	switch lv.Kind() {
	case reflect.Array, reflect.Slice:
		// if the slice contains 'interface' elements, then the element needs to be extracted directly to examine its type,
		// otherwise it will just resolve to 'interface'.
		var interfaceSlice []interface{}
		if reflect.TypeOf(l).Elem().Kind() == reflect.Interface {
			interfaceSlice = l.([]interface{})
		}

		for i := 0; i < lv.Len(); i++ {
			var lvv reflect.Value
			if interfaceSlice != nil {
				lvv = reflect.ValueOf(interfaceSlice[i])
			} else {
				lvv = lv.Index(i)
			}

			switch lvv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				switch vv.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					if vv.Int() == lvv.Int() {
						return true, nil
					}
				}
			case reflect.Float32, reflect.Float64:
				switch vv.Kind() {
				case reflect.Float32, reflect.Float64:
					if vv.Float() == lvv.Float() {
						return true, nil
					}
				}
			case reflect.String:
				if vv.Type() == lvv.Type() && vv.String() == lvv.String() {
					return true, nil
				}
			}
		}
	case reflect.String:
		if vv.Type() == lv.Type() && strings.Contains(lv.String(), vv.String()) {
			return true, nil
		}
	}

	return false, nil
}

// loop accepts varying parameters and differs its behavior. If given one
// parameter, loop will return a goroutine that begins at 0 and loops until the
// given int, increasing the index by 1 each iteration. If given two parameters,
// loop will return a goroutine that begins at the first parameter and loops
// up to but not including the second parameter.
//
//    // Prints 0 1 2 3 4
// 		for _, i := range loop(5) {
// 			print(i)
// 		}
//
//    // Prints 5 6 7
// 		for _, i := range loop(5, 8) {
// 			print(i)
// 		}
//
func loop(ints ...int64) (<-chan int64, error) {
	var start, stop int64
	switch len(ints) {
	case 1:
		start, stop = 0, ints[0]
	case 2:
		start, stop = ints[0], ints[1]
	default:
		return nil, fmt.Errorf("loop: wrong number of arguments, expected 1 or 2"+
			", but got %d", len(ints))
	}

	ch := make(chan int64)

	go func() {
		for i := start; i < stop; i++ {
			ch <- i
		}
		close(ch)
	}()

	return ch, nil
}

// join is a version of strings.Join that can be piped
func join(sep string, a []string) (string, error) {
	return strings.Join(a, sep), nil
}

// TrimSpace is a version of strings.TrimSpace that can be piped
func trimSpace(s string) (string, error) {
	return strings.TrimSpace(s), nil
}

// parseBool parses a string into a boolean
func parseBool(s string) (bool, error) {
	if s == "" {
		return false, nil
	}

	result, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("parseBool: %s", err)
	}
	return result, nil
}

// parseFloat parses a string into a base 10 float
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0.0, nil
	}

	result, err := strconv.ParseFloat(s, 10)
	if err != nil {
		return 0, fmt.Errorf("parseFloat: %s", err)
	}
	return result, nil
}

// parseInt parses a string into a base 10 int
func parseInt(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}

	result, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parseInt: %s", err)
	}
	return result, nil
}

// parseJSON returns a structure for valid JSON
func parseJSON(s string) (interface{}, error) {
	if s == "" {
		return map[string]interface{}{}, nil
	}

	var data interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil, err
	}
	return data, nil
}

// parseUint parses a string into a base 10 int
func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}

	result, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parseUint: %s", err)
	}
	return result, nil
}

// plugin executes a subprocess as the given command string. It is assumed the
// resulting command returns JSON which is then parsed and returned as the
// value for use in the template.
func plugin(name string, args ...string) (string, error) {
	if name == "" {
		return "", nil
	}

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

	// Strip and trim each arg or else some plugins get confused with the newline
	// characters
	jsons := make([]string, 0, len(args))
	for _, arg := range args {
		if v := strings.TrimSpace(arg); v != "" {
			jsons = append(jsons, v)
		}
	}

	cmd := exec.Command(name, jsons...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("exec %q: %s\n\nstdout:\n\n%s\n\nstderr:\n\n%s",
			name, err, stdout.Bytes(), stderr.Bytes())
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				return "", fmt.Errorf("exec %q: failed to kill", name)
			}
		}
		<-done // Allow the goroutine to exit
		return "", fmt.Errorf("exec %q: did not finish", name)
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("exec %q: %s\n\nstdout:\n\n%s\n\nstderr:\n\n%s",
				name, err, stdout.Bytes(), stderr.Bytes())
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// replaceAll replaces all occurrences of a value in a string with the given
// replacement value.
func replaceAll(f, t, s string) (string, error) {
	return strings.Replace(s, f, t, -1), nil
}

// regexReplaceAll replaces all occurrences of a regular expression with
// the given replacement value.
func regexReplaceAll(re, pl, s string) (string, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return "", err
	}
	return compiled.ReplaceAllString(s, pl), nil
}

// regexMatch returns true or false if the string matches
// the given regular expression
func regexMatch(re, s string) (bool, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(s), nil
}

// split is a version of strings.Split that can be piped
func split(sep, s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}, nil
	}
	return strings.Split(s, sep), nil
}

// timestamp returns the current UNIX timestamp in UTC. If an argument is
// specified, it will be used to format the timestamp.
func timestamp(s ...string) (string, error) {
	switch len(s) {
	case 0:
		return now().Format(time.RFC3339), nil
	case 1:
		if s[0] == "unix" {
			return strconv.FormatInt(now().Unix(), 10), nil
		}
		return now().Format(s[0]), nil
	default:
		return "", fmt.Errorf("timestamp: wrong number of arguments, expected 0 or 1"+
			", but got %d", len(s))
	}
}

// toLower converts the given string (usually by a pipe) to lowercase.
func toLower(s string) (string, error) {
	return strings.ToLower(s), nil
}

// toJSON converts the given structure into a deeply nested JSON string.
func toJSON(i interface{}) (string, error) {
	result, err := json.Marshal(i)
	if err != nil {
		return "", fmt.Errorf("toJSON: %s", err)
	}
	return string(bytes.TrimSpace(result)), err
}

// toJSONPretty converts the given structure into a deeply nested pretty JSON
// string.
func toJSONPretty(m map[string]interface{}) (string, error) {
	result, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("toJSONPretty: %s", err)
	}
	return string(bytes.TrimSpace(result)), err
}

// toTitle converts the given string (usually by a pipe) to titlecase.
func toTitle(s string) (string, error) {
	return strings.Title(s), nil
}

// toUpper converts the given string (usually by a pipe) to uppercase.
func toUpper(s string) (string, error) {
	return strings.ToUpper(s), nil
}

// toYAML converts the given structure into a deeply nested YAML string.
func toYAML(m map[string]interface{}) (string, error) {
	result, err := yaml.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("toYAML: %s", err)
	}
	return string(bytes.TrimSpace(result)), nil
}

// toTOML converts the given structure into a deeply nested TOML string.
func toTOML(m map[string]interface{}) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := toml.NewEncoder(buf)
	if err := enc.Encode(m); err != nil {
		return "", fmt.Errorf("toTOML: %s", err)
	}
	result, err := ioutil.ReadAll(buf)
	if err != nil {
		return "", fmt.Errorf("toTOML: %s", err)
	}
	return string(bytes.TrimSpace(result)), nil
}

// add returns the sum of a and b.
func add(b, a interface{}) (interface{}, error) {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	switch av.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Int() + bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Int() + int64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Int()) + bv.Float(), nil
		default:
			return nil, fmt.Errorf("add: unknown type for %q (%T)", bv, b)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int64(av.Uint()) + bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Uint() + bv.Uint(), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Uint()) + bv.Float(), nil
		default:
			return nil, fmt.Errorf("add: unknown type for %q (%T)", bv, b)
		}
	case reflect.Float32, reflect.Float64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Float() + float64(bv.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Float() + float64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return av.Float() + bv.Float(), nil
		default:
			return nil, fmt.Errorf("add: unknown type for %q (%T)", bv, b)
		}
	default:
		return nil, fmt.Errorf("add: unknown type for %q (%T)", av, a)
	}
}

// subtract returns the difference of b from a.
func subtract(b, a interface{}) (interface{}, error) {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	switch av.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Int() - bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Int() - int64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Int()) - bv.Float(), nil
		default:
			return nil, fmt.Errorf("subtract: unknown type for %q (%T)", bv, b)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int64(av.Uint()) - bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Uint() - bv.Uint(), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Uint()) - bv.Float(), nil
		default:
			return nil, fmt.Errorf("subtract: unknown type for %q (%T)", bv, b)
		}
	case reflect.Float32, reflect.Float64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Float() - float64(bv.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Float() - float64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return av.Float() - bv.Float(), nil
		default:
			return nil, fmt.Errorf("subtract: unknown type for %q (%T)", bv, b)
		}
	default:
		return nil, fmt.Errorf("subtract: unknown type for %q (%T)", av, a)
	}
}

// multiply returns the product of a and b.
func multiply(b, a interface{}) (interface{}, error) {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	switch av.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Int() * bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Int() * int64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Int()) * bv.Float(), nil
		default:
			return nil, fmt.Errorf("multiply: unknown type for %q (%T)", bv, b)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int64(av.Uint()) * bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Uint() * bv.Uint(), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Uint()) * bv.Float(), nil
		default:
			return nil, fmt.Errorf("multiply: unknown type for %q (%T)", bv, b)
		}
	case reflect.Float32, reflect.Float64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Float() * float64(bv.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Float() * float64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return av.Float() * bv.Float(), nil
		default:
			return nil, fmt.Errorf("multiply: unknown type for %q (%T)", bv, b)
		}
	default:
		return nil, fmt.Errorf("multiply: unknown type for %q (%T)", av, a)
	}
}

// divide returns the division of b from a.
func divide(b, a interface{}) (interface{}, error) {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	switch av.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Int() / bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Int() / int64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Int()) / bv.Float(), nil
		default:
			return nil, fmt.Errorf("divide: unknown type for %q (%T)", bv, b)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int64(av.Uint()) / bv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Uint() / bv.Uint(), nil
		case reflect.Float32, reflect.Float64:
			return float64(av.Uint()) / bv.Float(), nil
		default:
			return nil, fmt.Errorf("divide: unknown type for %q (%T)", bv, b)
		}
	case reflect.Float32, reflect.Float64:
		switch bv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return av.Float() / float64(bv.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return av.Float() / float64(bv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return av.Float() / bv.Float(), nil
		default:
			return nil, fmt.Errorf("divide: unknown type for %q (%T)", bv, b)
		}
	default:
		return nil, fmt.Errorf("divide: unknown type for %q (%T)", av, a)
	}
}

// addDependency adds the given Dependency to the map.
func addDependency(m map[string]dep.Dependency, d dep.Dependency) {
	if _, ok := m[d.HashCode()]; !ok {
		m[d.HashCode()] = d
	}
}
