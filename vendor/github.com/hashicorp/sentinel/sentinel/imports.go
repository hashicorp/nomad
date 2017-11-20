package sentinel

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/sentinel-sdk"
	sentinelrpc "github.com/hashicorp/sentinel-sdk/rpc"
	"github.com/hashicorp/sentinel/imports/stdlib"
)

// Import defines an available import for Sentinel execution.
//
// Sentinel manages import lifecycle. For frequently-used imports, they
// will be reused and remain in memory. For imports that haven't been used
// recently, Sentinel may deallocate and reinitialize them on use.
type Import struct {
	// Func is called to create this import. This import may be reused
	// across multiple policy executions as long as their configuration
	// matches.
	//
	// If the return value implements io.Closer, Sentinel will call Close
	// when this import is no longer in use.
	//
	// If Func and Path are both specified, Func takes precedence.
	Func func() (sdk.Import, error)

	// The options below configure imports over an external plugin process.
	//
	// Path is the path to an external plugin process. This will be launched
	// when needed to dispense the import. Args and Env control
	// additional settings for the launched binary.
	Path string
	Args []string
	Env  []string

	// Config is the configuration for this import after it is loaded.
	// This is the default global config. A per-policy config may also
	// be specified on Policy. If a per-policy config is specified, it
	// will override this config completely.
	Config map[string]interface{}
}

// ImportsMap wraps a map of defined imports to provide helper APIs to
// make working with imports easier. These APIs are designed to work well
// with expected user configuration styles.
type ImportsMap map[string]*Import

// Configure will set the configuration for the given imports. If a
// configuration is set for an import that isn't in the map, it is ignored.
func (m ImportsMap) Configure(cfg map[string]map[string]interface{}) {
	for k, c := range cfg {
		if impt, ok := m[k]; ok {
			impt.Config = c
		}
	}
}

// Blacklist can be called to remove matching imports from the map. This is
// useful for excluding standard libraries from the imports map.
func (m ImportsMap) Blacklist(keys []string) {
	for _, k := range keys {
		delete(m, k)
	}
}

// Whitelist filters the map to only the given keys. This is useful for
// only allowing specific imports in the map.
func (m ImportsMap) Whitelist(keys []string) {
	keyMap := make(map[string]struct{})
	for _, k := range keys {
		keyMap[k] = struct{}{}
	}

	for k := range m {
		if _, ok := keyMap[k]; !ok {
			delete(m, k)
		}
	}
}

//-------------------------------------------------------------------
// Stdlib

// StdImports returns all of the standard library imports without any
// configuration. The caller can then use simple map delete to remove any
// unwanted imports.
//
// This is a reasonable default to start with when configuring imports. This
// will return a newly allocated map on each call.
func StdImports() map[string]*Import {
	m := make(map[string]*Import)
	for k, raw := range stdlib.Imports {
		f := raw // Have to make the copy since Go reuses the range var

		m[k] = &Import{
			Func: func() (sdk.Import, error) { return f(), nil },
		}
	}

	return m
}

//-------------------------------------------------------------------
// Importer

// sentinelImport is the internal structure that tracks available imports.
type sentinelImport struct {
	// RWMutex must be held for operations on this import.
	sync.RWMutex

	Import *Import                            // The import configuration
	Hash   uint64                             // The hash of the global configuration
	Ready  map[uint64]*sentinelImportInstance // Created instances, keyed by config hash

	// Fields for external process plugins.
	pluginClient *goplugin.Client
}

// sentinelImportInstance represents a single instance of an import.
// A plugin process can serve multiple instances of an import.
type sentinelImportInstance struct {
	Import   sdk.Import
	LastUsed int64
}

// Touch updates the last used time. This keeps this value alive.
func (i *sentinelImportInstance) Touch() {
	atomic.StoreInt64(&i.LastUsed, time.Now().Unix())
}

// Close releases all resources associated with an import. This will close
// any initialized instances and kill the plugin process if it exists.
func (i *sentinelImport) Close() error {
	i.Lock()
	defer i.Unlock()
	return i.closeLocked()
}

func (i *sentinelImport) closeLocked() error {
	var err error

	// Close all the instances if they implement io.Closer
	for k, instance := range i.Ready {
		// Delete the value so that the import could potentially be used again
		delete(i.Ready, k)

		if c, ok := instance.Import.(io.Closer); ok {
			if e := c.Close(); e != nil {
				err = multierror.Append(err, e)
			}
		}
	}

	// Kill the plugin process if we have one
	if i.pluginClient != nil {
		i.pluginClient.Kill()
		i.pluginClient = nil
	}

	return err
}

// checkPlugin checks the health of the plugin binary, killing it out
// if necessary.
//
// The write lock must be held when calling this.
func (i *sentinelImport) checkPlugin() {
	if i.pluginClient == nil {
		return
	}

	client, err := i.pluginClient.Client()
	if err != nil {
		return
	}

	if err := client.Ping(); err == nil {
		return
	}

	// Unhealthy. Close the import.
	i.closeLocked()
}

//-------------------------------------------------------------------
// Importer

// sentinelImporter is an importer.Importer that processes the imports for
// a single policy.
//
// Multiple importers can run concurrently for the same Sentinel instance
// and Policy implementation.
type sentinelImporter struct {
	Sentinel *Sentinel
	Policy   *Policy
}

// Close cleans up resources associated with this importer. It should
// be called when all evaluations using this importer are complete.
func (m *sentinelImporter) Close() error {
	return nil
}

func (m *sentinelImporter) Import(name string) (sdk.Import, error) {
	// Grab a read lock
	m.Sentinel.importsLock.RLock()
	defer m.Sentinel.importsLock.RUnlock()

	// Verify this is a valid import
	global, ok := m.Sentinel.imports[name]
	if !ok {
		return nil, fmt.Errorf("Import %q is not available", name)
	}
	config := global.Import.Config
	hash := global.Hash

	// Check if we have a policy-specific override
	policy, ok := m.Policy.imports[name]
	if ok {
		config = policy.Config
		hash = policy.Hash
	}

	// Check if we have this initialized already.
	global.RLock()
	v, ok := global.Ready[hash]
	if ok && global.pluginClient != nil {
		// If we found a ready-to-go instance, then we check the health
		// of the connection to ensure we don't return an unhealthy
		// RPC connection.
		client, err := global.pluginClient.Client()
		if err == nil {
			err = client.Ping()
		}

		if err != nil {
			// Something went wrong, mark that we didn't find the value.
			// This will force the reconnection below.
			ok = false
		}
	}
	global.RUnlock()
	if ok {
		v.Touch()
		return v.Import, nil
	}

	// Don't have it yet, grab a write lock so we can set the import.
	global.Lock()
	defer global.Unlock()

	// Check the plugin health (if there is one)
	global.checkPlugin()

	// Check if we raced and it exists now. If so, just return
	if v, ok := global.Ready[hash]; ok {
		v.Touch()
		return v.Import, nil
	}

	// Not initialized, create it
	impt, err := m.initImport(global)
	if err != nil {
		return nil, err
	}

	// Configure the import. If the configuration fails, we just return
	// and don't store the import since it isn't ready.
	if err := impt.Configure(config); err != nil {
		return nil, fmt.Errorf(
			"Error configuring import %q: %s", name, err)
	}

	// Store it
	if global.Ready == nil {
		global.Ready = make(map[uint64]*sentinelImportInstance)
	}
	instance := &sentinelImportInstance{Import: impt}
	instance.Touch()
	global.Ready[hash] = instance

	return impt, nil
}

func (m *sentinelImporter) initImport(impt *sentinelImport) (sdk.Import, error) {
	// If the import specified a factory function, we just call that.
	if impt.Import.Func != nil {
		return impt.Import.Func()
	}

	// The import specified a path, this is an external binary plugin.
	// If we don't have a plugin client then we need to launch the plugin
	// binary.
	if impt.pluginClient == nil {
		cmd := exec.Command(impt.Import.Path, impt.Import.Args...)
		if v := impt.Import.Env; v != nil {
			cmd.Env = v
		}

		impt.pluginClient = goplugin.NewClient(&goplugin.ClientConfig{
			HandshakeConfig:  sentinelrpc.Handshake,
			Plugins:          sentinelrpc.PluginMap,
			Cmd:              cmd,
			AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		})
	}

	// Grab the client
	client, err := impt.pluginClient.Client()
	if err != nil {
		return nil, err
	}

	// Get the plugin
	raw, err := client.Dispense(sentinelrpc.ImportPluginName)
	if err != nil {
		return nil, err
	}

	return raw.(sdk.Import), nil
}

//-------------------------------------------------------------------
// Import Reaping

const (
	// ImportReapTTL is the TTL that an instance of an import remains
	// allocated and configured. When this TTL runs out, the reaper will
	// deallocate that instance.
	//
	// This should be guaranteed to be longer than the maximum execution
	// time of a policy. If a reap operation occurs during policy execution,
	// the policy could error.
	ImportReapTTL = 1 * time.Minute
)

// importReaper reaps unused or expired imports. This should be started in
// a goroutine.
//
// The current import algorithm is O(N) where N is the number of import
// instances there are. We assume that N is relatively low and that policies
// won't instantiate too many distinct imports which makes the complexity
// of the reaper reasonable.
func (s *Sentinel) importReaper(ctx context.Context) {
	// Create a timer that takes an extremely long time. We set the
	// actual timer as the first part of the loop below.
	timer := time.NewTimer(24 * time.Hour)
	defer timer.Stop()

	for {
		// Reset the timer so it expires after the TTL again.
		timer.Reset(s.importReapTTL)

		select {
		case <-ctx.Done():
			// We're stopping
			return

		case <-timer.C:
			// Time to check for reapable imports
		}

		// expireTime is the time an import would be expired
		expireTime := time.Now().Add(-1 * s.importReapTTL)

		// Build the list of import instances to reap. This will typically be
		// zero. To build this list, we acquire a read lock since we assume
		// that we won't have to reap anything.
		var toReap []importReap
		s.importsLock.RLock()
		for _, impt := range s.imports {
			impt.RLock()

			for key, instance := range impt.Ready {
				lastUsed := time.Unix(atomic.LoadInt64(&instance.LastUsed), 0)
				if lastUsed.Before(expireTime) {
					toReap = append(toReap, importReap{
						Import: impt,
						Key:    key,
					})
				}
			}

			impt.RUnlock()
		}
		s.importsLock.RUnlock()

		// If we have nothing to reap, then just continue. This is likely.
		if len(toReap) == 0 {
			continue
		}

		// We have things to reap. We need to go through and acquire
		// the proper locks.
		for _, reap := range toReap {
			impt := reap.Import

			// Acquire the lock for a short period of time just to
			// delte this instance out of it. We grab a reference to we
			// can clean up.
			impt.Lock()
			instance, ok := impt.Ready[reap.Key]
			delete(impt.Ready, reap.Key)
			impt.Unlock()

			// If it no longer exists (strange), then just continue.
			if !ok {
				continue
			}

			// Close it if it implements the closer
			if c, ok := instance.Import.(io.Closer); ok {
				// TODO: log error
				c.Close()
			}
		}
	}
}

// importReap is a structure that stores what imports we should reap.
type importReap struct {
	Import *sentinelImport
	Key    uint64
}
