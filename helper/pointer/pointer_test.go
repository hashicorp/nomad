package pointer

import (
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func Test_Of(t *testing.T) {
	s := "hello"
	sPtr := Of(s)

	must.Eq(t, s, *sPtr)

	b := "bye"
	sPtr = &b
	must.NotEq(t, s, *sPtr)
}

func Test_Copy(t *testing.T) {
	orig := Of(1)
	dup := Copy(orig)
	orig = Of(7)
	must.EqOp(t, 7, *orig)
	must.EqOp(t, 1, *dup)
}

func Test_Compare(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		a := 1
		b := 2
		c := 1
		var n *int // nil
		must.False(t, Eq(&a, &b))
		must.True(t, Eq(&a, &c))
		must.False(t, Eq(nil, &a))
		must.False(t, Eq(n, &a))
		must.True(t, Eq(n, nil))
	})

	t.Run("string", func(t *testing.T) {
		a := "cat"
		b := "dog"
		c := "cat"
		var n *string

		must.False(t, Eq(&a, &b))
		must.True(t, Eq(&a, &c))
		must.False(t, Eq(nil, &a))
		must.False(t, Eq(n, &a))
		must.True(t, Eq(n, nil))
	})

	t.Run("duration", func(t *testing.T) {
		a := time.Duration(1)
		b := time.Duration(2)
		c := time.Duration(1)
		var n *time.Duration

		must.False(t, Eq(&a, &b))
		must.True(t, Eq(&a, &c))
		must.False(t, Eq(nil, &a))
		must.False(t, Eq(n, &a))
		must.True(t, Eq(n, nil))
	})
}
