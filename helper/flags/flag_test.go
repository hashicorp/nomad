package flags

import (
	"flag"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringFlag_implements(t *testing.T) {
	t.Parallel()

	var raw interface{}
	raw = new(StringFlag)
	if _, ok := raw.(flag.Value); !ok {
		t.Fatalf("StringFlag should be a Value")
	}
}

func TestStringFlagSet(t *testing.T) {
	t.Parallel()

	sv := new(StringFlag)
	err := sv.Set("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = sv.Set("bar")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := []string{"foo", "bar"}
	if !reflect.DeepEqual([]string(*sv), expected) {
		t.Fatalf("Bad: %#v", sv)
	}
}
func TestStringFlagSet_Append(t *testing.T) {
	t.Parallel()

	var (
		// A test to make sure StringFlag can replace AppendSliceValue
		// for autopilot flags inherited from Consul.
		hosts StringFlag
	)

	flagSet := flag.NewFlagSet("test", flag.PanicOnError)
	flagSet.Var(&hosts, "host", "host, specify more than once")

	args := []string{"-host", "foo", "-host", "bar", "-host", "baz"}
	err := flagSet.Parse(args)
	require.NoError(t, err)

	result := hosts.String()
	require.Equal(t, "foo,bar,baz", result)
}
