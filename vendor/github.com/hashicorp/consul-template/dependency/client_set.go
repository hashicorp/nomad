package dependency

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"sync"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-cleanhttp"
	rootcerts "github.com/hashicorp/go-rootcerts"
	vaultapi "github.com/hashicorp/vault/api"
)

// ClientSet is a collection of clients that dependencies use to communicate
// with remote services like Consul or Vault.
type ClientSet struct {
	sync.RWMutex

	vault  *vaultClient
	consul *consulClient
}

// consulClient is a wrapper around a real Consul API client.
type consulClient struct {
	client     *consulapi.Client
	httpClient *http.Client
}

// vaultClient is a wrapper around a real Vault API client.
type vaultClient struct {
	client     *vaultapi.Client
	httpClient *http.Client
}

// CreateConsulClientInput is used as input to the CreateConsulClient function.
type CreateConsulClientInput struct {
	Address      string
	Token        string
	AuthEnabled  bool
	AuthUsername string
	AuthPassword string
	SSLEnabled   bool
	SSLVerify    bool
	SSLCert      string
	SSLKey       string
	SSLCACert    string
	SSLCAPath    string
	ServerName   string
}

// CreateVaultClientInput is used as input to the CreateVaultClient function.
type CreateVaultClientInput struct {
	Address     string
	Token       string
	UnwrapToken bool
	SSLEnabled  bool
	SSLVerify   bool
	SSLCert     string
	SSLKey      string
	SSLCACert   string
	SSLCAPath   string
	ServerName  string
}

// NewClientSet creates a new client set that is ready to accept clients.
func NewClientSet() *ClientSet {
	return &ClientSet{}
}

// CreateConsulClient creates a new Consul API client from the given input.
func (c *ClientSet) CreateConsulClient(i *CreateConsulClientInput) error {
	log.Printf("[INFO] (clients) creating consul/api client")

	// Generate the default config
	consulConfig := consulapi.DefaultConfig()

	// Set the address
	if i.Address != "" {
		log.Printf("[DEBUG] (clients) setting consul address to %q", i.Address)
		consulConfig.Address = i.Address
	}

	// Configure the token
	if i.Token != "" {
		log.Printf("[DEBUG] (clients) setting consul token")
		consulConfig.Token = i.Token
	}

	// Add basic auth
	if i.AuthEnabled {
		log.Printf("[DEBUG] (clients) setting basic auth")
		consulConfig.HttpAuth = &consulapi.HttpBasicAuth{
			Username: i.AuthUsername,
			Password: i.AuthPassword,
		}
	}

	// This transport will attempt to keep connections open to the Consul server.
	transport := cleanhttp.DefaultPooledTransport()

	// Configure SSL
	if i.SSLEnabled {
		log.Printf("[DEBUG] (clients) enabling consul SSL")
		consulConfig.Scheme = "https"

		var tlsConfig tls.Config

		// Custom certificate or certificate and key
		if i.SSLCert != "" && i.SSLKey != "" {
			cert, err := tls.LoadX509KeyPair(i.SSLCert, i.SSLKey)
			if err != nil {
				return fmt.Errorf("client set: consul: %s", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		} else if i.SSLCert != "" {
			cert, err := tls.LoadX509KeyPair(i.SSLCert, i.SSLCert)
			if err != nil {
				return fmt.Errorf("client set: consul: %s", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		// Custom CA certificate
		if i.SSLCACert != "" || i.SSLCAPath != "" {
			rootConfig := &rootcerts.Config{
				CAFile: i.SSLCACert,
				CAPath: i.SSLCAPath,
			}
			if err := rootcerts.ConfigureTLS(&tlsConfig, rootConfig); err != nil {
				return fmt.Errorf("client set: consul configuring TLS failed: %s", err)
			}
		}

		// Construct all the certificates now
		tlsConfig.BuildNameToCertificate()

		// SSL verification
		if i.ServerName != "" {
			tlsConfig.ServerName = i.ServerName
			tlsConfig.InsecureSkipVerify = false
			log.Printf("[DEBUG] (clients) using explicit consul TLS server host name: %s", tlsConfig.ServerName)
		}
		if !i.SSLVerify {
			log.Printf("[WARN] (clients) disabling consul SSL verification")
			tlsConfig.InsecureSkipVerify = true
		}

		// Save the TLS config on our transport
		transport.TLSClientConfig = &tlsConfig
	}

	// Setup the new transport
	consulConfig.HttpClient.Transport = transport

	// Create the API client
	client, err := consulapi.NewClient(consulConfig)
	if err != nil {
		return fmt.Errorf("client set: consul: %s", err)
	}

	// Save the data on ourselves
	c.consul = &consulClient{
		client:     client,
		httpClient: consulConfig.HttpClient,
	}

	return nil
}

func (c *ClientSet) CreateVaultClient(i *CreateVaultClientInput) error {
	log.Printf("[INFO] (clients) creating vault/api client")

	// Generate the default config
	vaultConfig := vaultapi.DefaultConfig()

	// Set the address
	if i.Address != "" {
		log.Printf("[DEBUG] (clients) setting vault address to %q", i.Address)
		vaultConfig.Address = i.Address
	}

	// This transport will attempt to keep connections open to the Vault server.
	transport := cleanhttp.DefaultPooledTransport()

	// Configure SSL
	if i.SSLEnabled {
		log.Printf("[DEBUG] (clients) enabling vault SSL")
		var tlsConfig tls.Config

		// Custom certificate or certificate and key
		if i.SSLCert != "" && i.SSLKey != "" {
			cert, err := tls.LoadX509KeyPair(i.SSLCert, i.SSLKey)
			if err != nil {
				return fmt.Errorf("client set: vault: %s", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		} else if i.SSLCert != "" {
			cert, err := tls.LoadX509KeyPair(i.SSLCert, i.SSLCert)
			if err != nil {
				return fmt.Errorf("client set: vault: %s", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		// Custom CA certificate
		if i.SSLCACert != "" || i.SSLCAPath != "" {
			rootConfig := &rootcerts.Config{
				CAFile: i.SSLCACert,
				CAPath: i.SSLCAPath,
			}
			if err := rootcerts.ConfigureTLS(&tlsConfig, rootConfig); err != nil {
				return fmt.Errorf("client set: vault configuring TLS failed: %s", err)
			}
		}

		// Construct all the certificates now
		tlsConfig.BuildNameToCertificate()

		// SSL verification
		if i.ServerName != "" {
			tlsConfig.ServerName = i.ServerName
			tlsConfig.InsecureSkipVerify = false
			log.Printf("[DEBUG] (clients) using explicit vault TLS server host name: %s", tlsConfig.ServerName)
		}
		if !i.SSLVerify {
			log.Printf("[WARN] (clients) disabling vault SSL verification")
			tlsConfig.InsecureSkipVerify = true
		}

		// Save the TLS config on our transport
		transport.TLSClientConfig = &tlsConfig
	}

	// Setup the new transport
	vaultConfig.HttpClient.Transport = transport

	// Create the client
	client, err := vaultapi.NewClient(vaultConfig)
	if err != nil {
		return fmt.Errorf("client set: vault: %s", err)
	}

	// Set the token if given
	if i.Token != "" {
		log.Printf("[DEBUG] (clients) setting vault token")
		client.SetToken(i.Token)
	}

	// Check if we are unwrapping
	if i.UnwrapToken {
		log.Printf("[INFO] (clients) unwrapping vault token")
		secret, err := client.Logical().Unwrap(i.Token)
		if err != nil {
			return fmt.Errorf("client set: vault unwrap: %s", err)
		}

		if secret == nil {
			return fmt.Errorf("client set: vault unwrap: no secret")
		}

		if secret.Auth == nil {
			return fmt.Errorf("client set: vault unwrap: no secret auth")
		}

		if secret.Auth.ClientToken == "" {
			return fmt.Errorf("client set: vault unwrap: no token returned")
		}

		client.SetToken(secret.Auth.ClientToken)
	}

	// Save the data on ourselves
	c.vault = &vaultClient{
		client:     client,
		httpClient: vaultConfig.HttpClient,
	}

	return nil
}

// Consul returns the Consul client for this clientset, or an error if no
// Consul client has been set.
func (c *ClientSet) Consul() (*consulapi.Client, error) {
	c.RLock()
	defer c.RUnlock()

	if c.consul == nil {
		return nil, fmt.Errorf("clientset: missing consul client")
	}
	cp := new(consulapi.Client)
	*cp = *c.consul.client
	return cp, nil
}

// Vault returns the Vault client for this clientset, or an error if no
// Vault client has been set.
func (c *ClientSet) Vault() (*vaultapi.Client, error) {
	c.RLock()
	defer c.RUnlock()

	if c.vault == nil {
		return nil, fmt.Errorf("clientset: missing vault client")
	}
	cp := new(vaultapi.Client)
	*cp = *c.vault.client
	return cp, nil
}

// Stop closes all idle connections for any attached clients.
func (c *ClientSet) Stop() {
	c.Lock()
	defer c.Unlock()

	if c.consul != nil {
		c.consul.httpClient.Transport.(*http.Transport).CloseIdleConnections()
	}

	if c.vault != nil {
		c.vault.httpClient.Transport.(*http.Transport).CloseIdleConnections()
	}
}
