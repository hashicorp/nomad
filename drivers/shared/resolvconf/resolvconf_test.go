package resolvconf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvConf_Content(t *testing.T) {
	cases := []struct {
		name string
		in   *ResolvConf
		out  []byte
	}{
		{
			name: "empty",
			in:   &ResolvConf{},
			out:  nil,
		},
		{
			name: "severs",
			in:   &ResolvConf{servers: []string{"8.8.8.8", "8.8.4.4"}},
			out:  []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n"),
		},
		{
			name: "ipv6 servers",
			in:   &ResolvConf{servers: []string{"2606:4700:4700::1111", "2606:4700:4700::1001"}},
			out:  []byte("nameserver 2606:4700:4700::1111\nnameserver 2606:4700:4700::1001\n"),
		},
		{
			name: "search servers",
			in:   &ResolvConf{servers: []string{"1.1.1.1", "1.0.0.1"}, searches: []string{"infra.nomad", "local.test"}},
			out:  []byte("search infra.nomad local.test\nnameserver 1.1.1.1\nnameserver 1.0.0.1\n"),
		},
		{
			name: "full example",
			in: &ResolvConf{
				servers:  []string{"1.1.1.1", "1.0.0.1"},
				searches: []string{"infra.nomad", "local.test"},
				options:  []string{"ndots:2", "edns0"},
			},
			out: []byte("search infra.nomad local.test\nnameserver 1.1.1.1\nnameserver 1.0.0.1\noptions ndots:2 edns0\n"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(childT *testing.T) {
			childT.Parallel()
			require.Equal(childT, c.out, c.in.Content())
		})
	}
}
