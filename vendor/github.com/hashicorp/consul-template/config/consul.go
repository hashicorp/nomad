package config

import "fmt"

// ConsulConfig contains the configurations options for connecting to a
// Consul cluster.
type ConsulConfig struct {
	// Address is the address of the Consul server. It may be an IP or FQDN.
	Address *string

	// Auth is the HTTP basic authentication for communicating with Consul.
	Auth *AuthConfig `mapstructure:"auth"`

	// Retry is the configuration for specifying how to behave on failure.
	Retry *RetryConfig `mapstructure:"retry"`

	// SSL indicates we should use a secure connection while talking to
	// Consul. This requires Consul to be configured to serve HTTPS.
	SSL *SSLConfig `mapstructure:"ssl"`

	// Token is the token to communicate with Consul securely.
	Token *string
}

// DefaultConsulConfig returns a configuration that is populated with the
// default values.
func DefaultConsulConfig() *ConsulConfig {
	return &ConsulConfig{
		Address: stringFromEnv("CONSUL_HTTP_ADDR"),
		Auth:    DefaultAuthConfig(),
		Retry:   DefaultRetryConfig(),
		SSL:     DefaultSSLConfig(),
		Token:   stringFromEnv("CONSUL_TOKEN", "CONSUL_HTTP_TOKEN"),
	}
}

// Copy returns a deep copy of this configuration.
func (c *ConsulConfig) Copy() *ConsulConfig {
	if c == nil {
		return nil
	}

	var o ConsulConfig

	o.Address = c.Address

	if c.Auth != nil {
		o.Auth = c.Auth.Copy()
	}

	if c.Retry != nil {
		o.Retry = c.Retry.Copy()
	}

	if c.SSL != nil {
		o.SSL = c.SSL.Copy()
	}

	o.Token = c.Token

	return &o
}

// Merge combines all values in this configuration with the values in the other
// configuration, with values in the other configuration taking precedence.
// Maps and slices are merged, most other values are overwritten. Complex
// structs define their own merge functionality.
func (c *ConsulConfig) Merge(o *ConsulConfig) *ConsulConfig {
	if c == nil {
		if o == nil {
			return nil
		}
		return o.Copy()
	}

	if o == nil {
		return c.Copy()
	}

	r := c.Copy()

	if o.Address != nil {
		r.Address = o.Address
	}

	if o.Auth != nil {
		r.Auth = r.Auth.Merge(o.Auth)
	}

	if o.Retry != nil {
		r.Retry = r.Retry.Merge(o.Retry)
	}

	if o.SSL != nil {
		r.SSL = r.SSL.Merge(o.SSL)
	}

	if o.Token != nil {
		r.Token = o.Token
	}

	return r
}

// Finalize ensures there no nil pointers.
func (c *ConsulConfig) Finalize() {
	if c.Address == nil {
		c.Address = String("")
	}

	if c.Auth == nil {
		c.Auth = DefaultAuthConfig()
	}
	c.Auth.Finalize()

	if c.Retry == nil {
		c.Retry = DefaultRetryConfig()
	}
	c.Retry.Finalize()

	if c.SSL == nil {
		c.SSL = DefaultSSLConfig()
	}
	c.SSL.Finalize()

	if c.Token == nil {
		c.Token = String("")
	}
}

// GoString defines the printable version of this struct.
func (c *ConsulConfig) GoString() string {
	if c == nil {
		return "(*ConsulConfig)(nil)"
	}

	return fmt.Sprintf("&ConsulConfig{"+
		"Address:%s, "+
		"Auth:%#v, "+
		"Retry:%#v, "+
		"SSL:%#v, "+
		"Token:%t"+
		"}",
		StringGoString(c.Address),
		c.Auth,
		c.Retry,
		c.SSL,
		StringPresent(c.Token),
	)
}
