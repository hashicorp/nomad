package datalog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDatalog_Basics(t *testing.T) {
	db := NewDB()

	// facts in the database are idempotent
	db.Assert("foo", "parent(bob,charlie).")
	db.Assert("bar", "parent(bob,charlie).")
	as := db.Query("parent(A,B)?")
	require.Equal(t, []string{"parent(bob, charlie)"}, as)

	// names are just for error reporting
	db.Assert("foo", "parent(alice,charlie).")
	db.Assert("foo", "parent(charlie,dan).")
	db.Assert("foo", "parent(charlie,erin).")

	as = db.Query("parent(A,dan)?")
	require.Equal(t, 1, len(as))
	require.Contains(t, as, "parent(charlie, dan)")

	as = db.Query("parent(charlie, A)?")
	require.Contains(t, as, "parent(charlie, dan)")
	require.Contains(t, as, "parent(charlie, erin)")

	// both rules define ancestor
	db.Assert("foo", "ancestor(A, B) :- parent(A, B).")
	db.Assert("foo", "ancestor(A, B) :- parent(A, C), ancestor(C, B).")
	as = db.Query("ancestor(alice, X)?")
	require.Contains(t, as, "ancestor(alice, charlie)")
	require.Contains(t, as, "ancestor(alice, dan)")
	require.Contains(t, as, "ancestor(alice, erin)")

	// empty result
	as = db.Query("ancestor(bob, alice)?")
	require.Empty(t, as)
}

func TestDatalog_lines(t *testing.T) {
	ls := lines(`
thing(foo, bar).
other_thing(foo, bar).
thing(foo, A)?
`)

	require.Equal(t, []string{
		"thing(foo, bar).",
		"other_thing(foo, bar).",
		"thing(foo, A)?",
	}, ls)
}
