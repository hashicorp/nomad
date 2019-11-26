module github.com/hashicorp/consul-template

go 1.12

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878 // indirect
	github.com/frankban/quicktest v1.4.0 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/hashicorp/consul/api v1.2.0
	github.com/hashicorp/consul/sdk v0.2.0
	github.com/hashicorp/go-gatedio v0.5.0
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/go-rootcerts v1.0.1
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/logutils v1.0.0
	github.com/hashicorp/memberlist v0.1.4 // indirect
	github.com/hashicorp/serf v0.8.3 // indirect
	github.com/hashicorp/vault/api v1.0.5-0.20190730042357-746c0b111519
	github.com/mattn/go-shellwords v1.0.5
	github.com/miekg/dns v1.1.15 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/pierrec/lz4 v2.2.5+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80 // indirect
	golang.org/x/sys v0.0.0-20190730183949-1393eb018365 // indirect
	golang.org/x/text v0.3.2 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/golang/lint => golang.org/x/lint v0.0.0-20190409202823-959b441ac422

replace sourcegraph.com/sourcegraph/go-diff => github.com/sourcegraph/go-diff v0.5.1
