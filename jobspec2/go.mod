module github.com/hashicorp/nomad/jobspec2

go 1.26.5

// jobspec2 is built using the current source of the API module.
replace github.com/hashicorp/nomad/api => ../api

require (
	github.com/hashicorp/go-cty-funcs v0.1.0
	github.com/hashicorp/hcl/v2 v2.20.2-nomad-1
	github.com/hashicorp/nomad/api v0.0.0-20260814142628-f3fe893c53d2
	github.com/mitchellh/reflectwalk v1.0.2
	github.com/shoenig/test v1.13.2
	github.com/stretchr/testify v1.12.0
	github.com/zclconf/go-cty v1.19.0
	github.com/zclconf/go-cty-yaml v1.2.0
)

require (
	github.com/agext/levenshtein v1.2.1 // indirect
	github.com/apparentlymart/go-cidr v1.1.0 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/apparentlymart/go-textseg/v17 v17.0.1 // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/hashicorp/cronexpr v1.1.3 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
