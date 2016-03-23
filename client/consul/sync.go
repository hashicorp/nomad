package consul

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	consul "github.com/hashicorp/consul/api"

	"github.com/hashicorp/nomad/nomad/structs"
)

type ConsulService struct {
	client *consul.Client

	task *structs.Task

	services map[string]*consul.AgentService
	checks   map[string][]*consul.AgentCheck

	logger     *log.Logger
	shutdownCh chan struct{}
}

type ConsulConfig struct {
	Addr      string
	Token     string
	Auth      string
	EnableSSL bool
	VerifySSL bool
}

const (
	syncInterval = 5 * time.Second
)

func NewConsulService(config *ConsulConfig, logger *log.Logger, task *structs.Task) (*ConsulService, error) {
	var err error
	var c *consul.Client
	cfg := consul.DefaultConfig()
	if config.Addr != "" {
		cfg.Address = config.Addr
	}
	if config.Token != "" {
		cfg.Token = config.Token
	}
	if config.Auth != "" {
		var username, password string
		if strings.Contains(config.Auth, ":") {
			split := strings.SplitN(config.Auth, ":", 2)
			username = split[0]
			password = split[1]
		} else {
			username = config.Auth
		}

		cfg.HttpAuth = &consul.HttpBasicAuth{
			Username: username,
			Password: password,
		}
	}
	if config.EnableSSL {
		cfg.Scheme = "https"
	}
	if config.EnableSSL && !config.VerifySSL {
		cfg.HttpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	if c, err = consul.NewClient(cfg); err != nil {
		return nil, err
	}
	consulService := ConsulService{
		client: c,
		task:   task,

		logger: logger,

		shutdownCh: make(chan struct{}),
	}

	return &consulService, nil
}

func (c *ConsulService) Register(task *structs.Task) error {
	c.task = task
	return nil
}

func (c *ConsulService) Deregister() error {
	return nil
}

func (c *ConsulService) Update(task *structs.Task) error {
	c.Update(task)
	return nil
}

func (c *ConsulService) createService(service *structs.Service) (*consul.AgentService, error) {
	host, port := c.task.FindHostAndPortFor(service.PortLabel)
	if host == "" {
		return nil, fmt.Errorf("host for the service %q  couldn't be found", service.Name)
	}

	if port == 0 {
		return nil, fmt.Errorf("port for the service %q  couldn't be found", service.Name)
	}
	srv := consul.AgentService{
		ID:      service.ID,
		Service: service.Name,
		Tags:    service.Tags,
		Address: host,
		Port:    port,
	}
	return &srv, nil
}

func (c *ConsulService) Sync() {
	sync := time.After(syncInterval)

	for {
		select {
		case <-sync:
			if err := c.performSync; err != nil {
				c.logger.Printf("[DEBUG] consul: error in syncing task %q: %v", c.task.Name, err)
			}
			sync = time.After(syncInterval)
		case <-c.shutdownCh:
			c.logger.Printf("[INFO] consul: shutting down sync for task %q", c.task.Name)
			return
		}
	}
}

func (c *ConsulService) performSync() error {
	return nil
}
