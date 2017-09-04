package command

import (
	"flag"
	"reflect"
	"sort"
	"testing"
)

func TestMeta_FlagSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Flags    FlagSetFlags
		Expected []string
	}{
		{
			FlagSetNone,
			[]string{},
		},
		{
			FlagSetClient,
			[]string{
				"address",
				"no-color",
				"region",
				"namespace",
				"ca-cert",
				"ca-path",
				"client-cert",
				"client-key",
				"insecure",
				"tls-skip-verify",
			},
		},
	}

	for i, tc := range cases {
		var m Meta
		fs := m.FlagSet("foo", tc.Flags)

		actual := make([]string, 0, 0)
		fs.VisitAll(func(f *flag.Flag) {
			actual = append(actual, f.Name)
		})
		sort.Strings(actual)
		sort.Strings(tc.Expected)

		if !reflect.DeepEqual(actual, tc.Expected) {
			t.Fatalf("%d: flags: %#v\n\nExpected: %#v\nGot: %#v",
				i, tc.Flags, tc.Expected, actual)
		}
	}
}
