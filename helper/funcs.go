// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"crypto/sha512"
	"fmt"
	"maps"
	"math"
	"net/http"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/hcl/hcl/ast"
)

// validUUID is used to check if a given string looks like a UUID
var validUUID = regexp.MustCompile(`(?i)^[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}$`)

// validInterpVarKey matches valid dotted variable names for interpolation. The
// string must begin with one or more non-dot characters which may be followed
// by sequences containing a dot followed by a one or more non-dot characters.
var validInterpVarKey = regexp.MustCompile(`^[^.]+(\.[^.]+)*$`)

// invalidFilename is the minimum set of characters which must be removed or
// replaced to produce a valid filename
var invalidFilename = regexp.MustCompile(`[/\\<>:"|?*]`)

// invalidFilenameNonASCII = invalidFilename plus all non-ASCII characters
var invalidFilenameNonASCII = regexp.MustCompile(`[[:^ascii:]/\\<>:"|?*]`)

// invalidFilenameStrict = invalidFilename plus additional punctuation
var invalidFilenameStrict = regexp.MustCompile(`[/\\<>:"|?*$()+=[\];#@~,&']`)

type Copyable[T any] interface {
	Copy() T
}

// IsUUID returns true if the given string is a valid UUID.
func IsUUID(str string) bool {
	const uuidLen = 36
	if len(str) != uuidLen {
		return false
	}

	return validUUID.MatchString(str)
}

// IsValidInterpVariable returns true if a valid dotted variable names for
// interpolation. The string must begin with one or more non-dot characters
// which may be followed by sequences containing a dot followed by a one or more
// non-dot characters.
func IsValidInterpVariable(str string) bool {
	return validInterpVarKey.MatchString(str)
}

// HashUUID takes an input UUID and returns a hashed version of the UUID to
// ensure it is well distributed.
func HashUUID(input string) (output string, hashed bool) {
	if !IsUUID(input) {
		return "", false
	}

	// Hash the input
	buf := sha512.Sum512([]byte(input))
	output = fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])

	return output, true
}

// UniqueMapSliceValues returns the union of values from each slice in a map[K][]V.
func UniqueMapSliceValues[K, V comparable](m map[K][]V) []V {
	s := set.New[V](0)
	for _, slice := range m {
		s.InsertAll(slice)
	}
	return s.List()
}

// IsSubset returns whether the smaller set of items is a subset of
// the larger. If the smaller set is not a subset, the offending elements are
// returned.
func IsSubset[T comparable](larger, smaller []T) (bool, []T) {
	l := set.From(larger)
	if l.ContainsAll(smaller) {
		return true, nil
	}
	s := set.From(smaller)
	return false, s.Difference(l).List()
}

// StringHasPrefixInSlice returns true if s starts with any prefix in list.
func StringHasPrefixInSlice(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// IsDisjoint returns whether first and second are disjoint sets, and the set of
// offending elements if not.
func IsDisjoint[T comparable](first, second []T) (bool, []T) {
	f, s := set.From(first), set.From(second)
	intersection := f.Intersect(s)
	if intersection.Size() > 0 {
		return false, intersection.List()
	}
	return true, nil
}

// DeepCopyMap creates a copy of m by calling Copy() on each value.
//
// If m is nil the return value is nil.
func DeepCopyMap[M ~map[K]V, K comparable, V Copyable[V]](m M) M {
	if m == nil {
		return nil
	}

	result := make(M, len(m))
	for k, v := range m {
		result[k] = v.Copy()
	}
	return result
}

// CopySlice creates a deep copy of s. For slices with elements that do not
// implement Copy(), use slices.Clone.
func CopySlice[S ~[]E, E Copyable[E]](s S) S {
	if s == nil {
		return nil
	}

	result := make(S, len(s))
	for i, v := range s {
		result[i] = v.Copy()
	}
	return result
}

// MergeMapStringString will merge two maps into one. If a duplicate key exists
// the value in the second map will replace the value in the first map. If both
// maps are empty or nil this returns an empty map.
func MergeMapStringString(m map[string]string, n map[string]string) map[string]string {
	if len(m) == 0 && len(n) == 0 {
		return map[string]string{}
	}
	if len(m) == 0 {
		return n
	}
	if len(n) == 0 {
		return m
	}

	result := maps.Clone(m)

	for k, v := range n {
		result[k] = v
	}

	return result
}

// CopyMapOfSlice creates a copy of m, making copies of each []V.
func CopyMapOfSlice[K comparable, V any](m map[K][]V) map[K][]V {
	l := len(m)
	if l == 0 {
		return nil
	}

	c := make(map[K][]V, l)
	for k, v := range m {
		c[k] = slices.Clone(v)
	}
	return c
}

// CleanEnvVar replaces all occurrences of illegal characters in an environment
// variable with the specified byte.
func CleanEnvVar(s string, r byte) string {
	b := []byte(s)
	for i, c := range b {
		switch {
		case c == '_':
		case c == '.':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case i > 0 && c >= '0' && c <= '9':
		default:
			// Replace!
			b[i] = r
		}
	}
	return string(b)
}

// CleanFilename replaces invalid characters in filename
func CleanFilename(filename string, replace string) string {
	clean := invalidFilename.ReplaceAllLiteralString(filename, replace)
	return clean
}

// CleanFilenameASCIIOnly replaces invalid and non-ASCII characters in filename
func CleanFilenameASCIIOnly(filename string, replace string) string {
	clean := invalidFilenameNonASCII.ReplaceAllLiteralString(filename, replace)
	return clean
}

// CleanFilenameStrict replaces invalid and punctuation characters in filename
func CleanFilenameStrict(filename string, replace string) string {
	clean := invalidFilenameStrict.ReplaceAllLiteralString(filename, replace)
	return clean
}

func CheckHCLKeys(node ast.Node, valid []string) error {
	var list *ast.ObjectList
	switch n := node.(type) {
	case *ast.ObjectList:
		list = n
	case *ast.ObjectType:
		list = n.List
	default:
		return fmt.Errorf("cannot check HCL keys of type %T", n)
	}

	validMap := make(map[string]struct{}, len(valid))
	for _, v := range valid {
		validMap[v] = struct{}{}
	}

	var result error
	for _, item := range list.Items {
		key := item.Keys[0].Token.Value().(string)
		if _, ok := validMap[key]; !ok {
			result = multierror.Append(result, fmt.Errorf(
				"invalid key: %s", key))
		}
	}

	return result
}

// UnusedKeys returns a pretty-printed error if any `hcl:",unusedKeys"` is not empty
func UnusedKeys(obj interface{}) error {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}
	return unusedKeysImpl([]string{}, val)
}

func unusedKeysImpl(path []string, val reflect.Value) error {
	stype := val.Type()
	for i := 0; i < stype.NumField(); i++ {
		ftype := stype.Field(i)
		fval := val.Field(i)
		tags := strings.Split(ftype.Tag.Get("hcl"), ",")
		name := tags[0]
		tags = tags[1:]

		if fval.Kind() == reflect.Ptr {
			fval = reflect.Indirect(fval)
		}

		// struct? recurse. Add the struct's key to the path
		if fval.Kind() == reflect.Struct {
			err := unusedKeysImpl(append([]string{name}, path...), fval)
			if err != nil {
				return err
			}
			continue
		}

		// Search the hcl tags for "unusedKeys"
		unusedKeys := false
		for _, p := range tags {
			if p == "unusedKeys" {
				unusedKeys = true
				break
			}
		}

		if unusedKeys {
			ks, ok := fval.Interface().([]string)
			if ok && len(ks) != 0 {
				ps := ""
				if len(path) > 0 {
					ps = strings.Join(path, ".") + " "
				}
				return fmt.Errorf("%sunexpected keys %s",
					ps,
					strings.Join(ks, ", "))
			}
		}
	}
	return nil
}

// RemoveEqualFold removes the first string that EqualFold matches. It updates xs in place
func RemoveEqualFold(xs *[]string, search string) {
	sl := *xs
	for i, x := range sl {
		if strings.EqualFold(x, search) {
			sl = append(sl[:i], sl[i+1:]...)
			if len(sl) == 0 {
				*xs = nil
			} else {
				*xs = sl
			}
			return
		}
	}
}

// CheckNamespaceScope ensures that the provided namespace is equal to
// or a parent of the requested namespaces. Returns requested namespaces
// which are not equal to or a child of the provided namespace.
func CheckNamespaceScope(provided string, requested []string) []string {
	var offending []string
	for _, ns := range requested {
		rel, err := filepath.Rel(provided, ns)
		if err != nil {
			offending = append(offending, ns)
			// If relative path requires ".." it's not a child
		} else if strings.Contains(rel, "..") {
			offending = append(offending, ns)
		}
	}
	if len(offending) > 0 {
		return offending
	}
	return nil
}

// StopFunc is used to stop a time.Timer created with NewSafeTimer
type StopFunc func()

// NewSafeTimer creates a time.Timer but does not panic if duration is <= 0.
//
// Using a time.Timer is recommended instead of time.After when it is necessary
// to avoid leaking goroutines (e.g. in a select inside a loop).
//
// Returns the time.Timer and also a StopFunc, forcing the caller to deal
// with stopping the time.Timer to avoid leaking a goroutine.
//
// Note: If creating a Timer that should do nothing until Reset is called, use
// NewStoppedTimer instead for safely creating the timer in a stopped state.
func NewSafeTimer(duration time.Duration) (*time.Timer, StopFunc) {
	if duration <= 0 {
		// Avoid panic by using the smallest positive value. This is close enough
		// to the behavior of time.After(0), which this helper is intended to
		// replace.
		// https://go.dev/play/p/EIkm9MsPbHY
		duration = 1
	}

	t := time.NewTimer(duration)
	cancel := func() {
		t.Stop()
	}

	return t, cancel
}

// NewStoppedTimer creates a time.Timer in a stopped state. This is useful when
// the actual wait time will computed and set later via Reset.
func NewStoppedTimer() (*time.Timer, StopFunc) {
	t, f := NewSafeTimer(math.MaxInt64)
	t.Stop()
	return t, f
}

// ConvertSlice takes the input slice and generates a new one using the
// supplied conversion function to covert the element. This is useful when
// converting a slice of strings to a slice of structs which wraps the string.
func ConvertSlice[A, B any](original []A, conversion func(a A) B) []B {
	result := make([]B, len(original))
	for i, element := range original {
		result[i] = conversion(element)
	}
	return result
}

// ConvertMap takes the input map and generates a new one using the supplied
// conversion function to convert the values. This is useful when converting one
// map to another using the same keys.
func ConvertMap[K comparable, A, B any](original map[K]A, conversion func(a A) B) map[K]B {
	result := make(map[K]B, len(original))
	for k, a := range original {
		result[k] = conversion(a)
	}
	return result
}

// IsMethodHTTP returns whether s is a known HTTP method, ignoring case.
func IsMethodHTTP(s string) bool {
	switch strings.ToUpper(s) {
	case http.MethodGet:
	case http.MethodHead:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodPatch:
	case http.MethodDelete:
	case http.MethodConnect:
	case http.MethodOptions:
	case http.MethodTrace:
	default:
		return false
	}
	return true
}

// EqualFunc represents a type implementing the Equal method.
type EqualFunc[A any] interface {
	Equal(A) bool
}

// ElementsEqual returns true if slices a and b contain the same elements (in
// no particular order) using the Equal function defined on their type for
// comparison.
func ElementsEqual[T EqualFunc[T]](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
OUTER:
	for _, item := range a {
		for _, other := range b {
			if item.Equal(other) {
				continue OUTER
			}
		}
		return false
	}
	return true
}

// SliceSetEq returns true if slices a and b contain the same elements (in no
// particular order), using '==' for comparison.
//
// Note: for pointers, consider implementing an Equal method and using
// ElementsEqual instead.
func SliceSetEq[T comparable](a, b []T) bool {
	lenA, lenB := len(a), len(b)
	if lenA != lenB {
		return false
	}

	if lenA > 10 {
		// avoid quadratic comparisons over large input
		return set.From(a).EqualSlice(b)
	}

OUTER:
	for _, item := range a {
		for _, other := range b {
			if item == other {
				continue OUTER
			}
		}
		return false
	}
	return true
}

// WithLock executes a function while holding a lock.
func WithLock(lock sync.Locker, f func()) {
	lock.Lock()
	defer lock.Unlock()
	f()
}

// Merge takes two variables and returns variable b in case a has zero value.
// For pointer values please use pointer.Merge.
func Merge[T comparable](a, b T) T {
	var zero T
	if a == zero {
		return b
	}
	return a
}
