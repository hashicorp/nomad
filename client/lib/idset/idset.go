package idset

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set"
	"golang.org/x/exp/slices"
)

// An ID is representative of a non-negative identifier of something like
// a CPU core ID, a NUMA node ID, etc.
type ID interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint
}

// A Set contains some IDs.
//
// See the List Format section of
// https://www.man7.org/linux/man-pages/man7/cpuset.7.html
// for more information on the syntax and utility of these sets.
type Set[T ID] struct {
	items *set.Set[T]
}

// Empty creates a fresh Set with no elements.
func Empty[T ID]() *Set[T] {
	return &Set[T]{
		items: set.New[T](0),
	}
}

var (
	numberRe = regexp.MustCompile(`^\d+$`)
	spanRe   = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

func atoi[T ID](s string) T {
	i, _ := strconv.Atoi(s)
	return T(i)
}

func order[T ID](a, b T) (T, T) {
	if a < b {
		return a, b
	}
	return b, a
}

func Parse[T ID](list string) *Set[T] {
	result := Empty[T]()

	add := func(s string) {
		s = strings.TrimSpace(s)
		switch {
		case numberRe.MatchString(s):
			result.items.Insert(atoi[T](s))
		case spanRe.MatchString(s):
			values := spanRe.FindStringSubmatch(s)
			low, high := order(atoi[T](values[1]), atoi[T](values[2]))
			for i := low; i <= high; i++ {
				result.items.Insert(T(i))
			}
		}
	}

	pieces := strings.Split(list, ",")
	for _, piece := range pieces {
		add(piece)
	}

	return result
}

func From[T, U ID](slice []U) *Set[T] {
	result := Empty[T]()
	for _, item := range slice {
		result.items.Insert(T(item))
	}
	return result
}

func (s *Set[T]) Contains(item T) bool {
	return s.items.Contains(item)
}

func (s *Set[T]) Insert(item T) {
	s.items.Insert(item)
}

func (s *Set[T]) Slice() []T {
	items := s.items.Slice()
	slices.Sort(items)
	return items
}

func (s *Set[T]) String() string {
	if s.items.Empty() {
		return ""
	}

	var parts []string
	ids := s.Slice()

	low, high := ids[0], ids[0]
	for i := 1; i < len(ids); i++ {
		switch {
		case ids[i] == high+1:
			high = ids[i]
			continue
		case low == high:
			parts = append(parts, fmt.Sprintf("%d", low))
		default:
			parts = append(parts, fmt.Sprintf("%d-%d", low, high))
		}
		low, high = ids[i], ids[i] // new range
	}

	if low == high {
		parts = append(parts, fmt.Sprintf("%d", low))
	} else {
		parts = append(parts, fmt.Sprintf("%d-%d", low, high))
	}

	return strings.Join(parts, ",")
}

func (s *Set[T]) ForEach(f func(id T) error) error {
	for _, id := range s.items.Slice() {
		if err := f(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Set[T]) Size() int {
	return s.items.Size()
}
