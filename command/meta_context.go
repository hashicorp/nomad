package command

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/nomad/api"
	"github.com/zclconf/go-cty/cty"
)

const (
	metaContextDirPath         = "nomad/context"
	metaContextFileExtension   = ".hcl"
	metaContextDefaultFileName = "_default"
)

type metaContextStorage struct {
	dir string
}

type MetaContextConfig struct {
	Context *ContextConfig `hcl:"context,block"`
}

type ContextConfig struct {
	Name          string `hcl:"name,label"`
	Address       string `hcl:"address"`
	Region        string `hcl:"region,optional"`
	Namespace     string `hcl:"namespace,optional"`
	Token         string `hcl:"token,optional"`
	CACert        string `hcl:"ca_cert,optional"`
	CAPath        string `hcl:"ca_path,optional"`
	ClientCert    string `hcl:"client_cert,optional"`
	ClientKey     string `hcl:"client_key,optional"`
	TLSServerName string `hcl:"tls_server_name,optional"`
	Insecure      bool   `hcl:"insecure,optional"`
}

// mergeMetaFlags
func (cc *ContextConfig) mergeMetaFlags(m Meta) {
	if m.flagAddress != "" {
		cc.Address = m.flagAddress
	}
	if m.region != "" {
		cc.Region = m.region
	}
	if m.namespace != "" {
		cc.Namespace = m.namespace
	}
	if m.token != "" {
		cc.Token = m.token
	}
	if m.caCert != "" {
		cc.CACert = m.caCert
	}
	if m.caPath != "" {
		cc.CAPath = m.caPath
	}
	if m.clientCert != "" {
		cc.ClientCert = m.clientCert
	}
	if m.clientKey != "" {
		cc.ClientKey = m.clientKey
	}
	if m.tlsServerName != "" {
		cc.TLSServerName = m.tlsServerName
	}
	cc.Insecure = m.insecure
}

// overlayAPIConfig overwrites the API config will any values found within
// the context config file.
func (cc *ContextConfig) overlayAPIConfig(cfg *api.Config) {
	if cc.Address != "" {
		cfg.Address = cc.Address
	}
	if cc.Region != "" {
		cfg.Region = cc.Region
	}
	if cc.Namespace != "" {
		cfg.Namespace = cc.Namespace
	}
	if cc.Token != "" {
		cfg.SecretID = cc.Token
	}
	if cc.CACert != "" {
		cfg.TLSConfig.CACert = cc.CACert
	}
	if cc.CAPath != "" {
		cfg.TLSConfig.CAPath = cc.CAPath
	}
	if cc.ClientCert != "" {
		cfg.TLSConfig.ClientCert = cc.ClientCert
	}
	if cc.ClientKey != "" {
		cfg.TLSConfig.ClientKey = cc.ClientKey
	}
	if cc.TLSServerName != "" {
		cfg.TLSConfig.TLSServerName = cc.TLSServerName
	}
	cfg.TLSConfig.Insecure = cc.Insecure
}

// newMetaContextStorage provides and handle on the context storage as well as
// ensure the base directory is created for use.
func newMetaContextStorage() (*metaContextStorage, error) {

	homeDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to lookup user home directory: %v", err)
	}

	s := metaContextStorage{dir: path.Join(homeDir, metaContextDirPath)}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return nil, err
	}
	return &s, nil
}

// Delete is used to remove the named context. If the context is not found or
// the file is not as expected, an error will be returned.
func (s *metaContextStorage) Delete(name string) error {

	ctxPath := s.contextFilePath(name)

	fileInfo, err := os.Stat(ctxPath)
	if err != nil {
		return fmt.Errorf("failed to find context file: %v", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("context file %q is dir, not file", fileInfo.Name())
	}

	return os.Remove(ctxPath)
}

// Get is used to return the decoded context as identified by the passed name.
func (s *metaContextStorage) Get(name string) (*MetaContextConfig, error) {
	return s.get(s.contextFilePath(name))
}

// GetDefault returns the decoded context file which has been set as the
// default. If no default has been set, both return values will be nil.
func (s *metaContextStorage) GetDefault() (*MetaContextConfig, error) {
	ctxPath := s.defaultContextFilePath()

	ctxConfig, err := s.get(ctxPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return nil, err
	}

	return ctxConfig, nil
}

// get reads and decode the supplied file path as a context configuration file.
func (s *metaContextStorage) get(filePath string) (*MetaContextConfig, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg MetaContextConfig

	if err := hclsimple.Decode(filePath, content, nil, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// List walks the local Nomad context directory, decoding each context file and
// eventually returning an array of all that were found. Any default linking is
// ignored to avoid duplicates. A single error is considered fatal, therefore
// meaning a bad file will break listing.
func (s *metaContextStorage) List() ([]*MetaContextConfig, error) {

	var contexts []*MetaContextConfig

	walkFn := func(filePath string, info os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		//
		if strings.HasPrefix(info.Name(), metaContextDefaultFileName) {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read context file: %v", err)
		}

		var cfg MetaContextConfig

		if err := hclsimple.Decode(info.Name(), content, nil, &cfg); err != nil {
			return fmt.Errorf("failed to decode context file: %v", err)
		}

		contexts = append(contexts, &cfg)
		return nil
	}

	if err := filepath.Walk(s.dir, walkFn); err != nil {
		return nil, err
	}
	return contexts, nil
}

// Set writes the context configuration to a file within the Nomad context
// directory on the local machine.
func (s *metaContextStorage) Set(cfg *ContextConfig) error {

	// If the configuration name is not set, we would just write a file named
	// ".hcl", so check to avoid this.
	if cfg.Name == "" {
		return errors.New("context name must be set")
	}

	hclFile := hclwrite.NewEmptyFile()
	rootBody := hclFile.Body()

	// In order to omit empty and unset values, we need to construct the HCL
	// body at a low level. Without doing this, we either need to use pointers
	// for all config parameters, or accept that we write empty strings to the
	// file.
	contextBody := rootBody.AppendNewBlock("context", []string{cfg.Name}).Body()

	// Check and add each config parameter as an attribute to the HCL block.
	if cfg.Address != "" {
		contextBody.SetAttributeValue("address", cty.StringVal(cfg.Address))
	}
	if cfg.Region != "" {
		contextBody.SetAttributeValue("region", cty.StringVal(cfg.Region))
	}
	if cfg.Namespace != "" {
		contextBody.SetAttributeValue("namespace", cty.StringVal(cfg.Namespace))
	}
	if cfg.Token != "" {
		contextBody.SetAttributeValue("token", cty.StringVal(cfg.Token))
	}
	if cfg.CACert != "" {
		contextBody.SetAttributeValue("ca_cert", cty.StringVal(cfg.CACert))
	}
	if cfg.CAPath != "" {
		contextBody.SetAttributeValue("ca_path", cty.StringVal(cfg.CAPath))
	}
	if cfg.ClientCert != "" {
		contextBody.SetAttributeValue("client_cert", cty.StringVal(cfg.ClientCert))
	}
	if cfg.ClientKey != "" {
		contextBody.SetAttributeValue("client_key", cty.StringVal(cfg.ClientKey))
	}
	if cfg.TLSServerName != "" {
		contextBody.SetAttributeValue("tls_server_name", cty.StringVal(cfg.TLSServerName))
	}
	contextBody.SetAttributeValue("insecure", cty.BoolVal(cfg.Insecure))

	// It is easy enough to format the HCL, so do this which means we have a
	// pretty output file.
	hclBytes := hclwrite.Format(hclFile.Bytes())

	return os.WriteFile(s.contextFilePath(cfg.Name), hclBytes, 0600)
}

// SetDefault
func (s *metaContextStorage) SetDefault(name string) error {
	src := s.contextFilePath(name)

	if _, err := os.Stat(src); err != nil {
		return err
	}

	defaultPath := s.defaultContextFilePath()

	// Attempt to create a symlink
	if err := os.RemoveAll(defaultPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete existing default context")
	}

	err := os.Symlink(src, defaultPath)
	if err != nil {
		return fmt.Errorf("failed to create default symlink")
	}

	// On Windows when creating a symlink the Windows API can incorrectly
	// return an error message when not running as Administrator even when the symlink
	// is correctly created.
	// Manually validate the symlink was correctly created before returning an error
	ln, ferr := os.Readlink(defaultPath)
	if ferr != nil {
		// symlink has not been created return the original error
		return err
	}

	if ln != src {
		return err
	}

	return nil
}

// UnsetDefault deletes the default context linking. It is safe to call this
// without checking a default has been previously set.
func (s *metaContextStorage) UnsetDefault() error {
	if err := os.Remove(s.defaultContextFilePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *metaContextStorage) contextFilePath(name string) string {
	return filepath.Join(s.dir, name+metaContextFileExtension)
}

func (s *metaContextStorage) defaultContextFilePath() string {
	return filepath.Join(s.dir, metaContextDefaultFileName+metaContextFileExtension)
}
