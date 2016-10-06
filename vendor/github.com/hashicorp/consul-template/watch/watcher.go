package watch

import (
	"fmt"
	"log"
	"sync"
	"time"

	dep "github.com/hashicorp/consul-template/dependency"
)

// RetryFunc is a function that defines the retry for a given watcher. The
// function parameter is the current retry (which might be nil), and the
// return value is the new retry. In this way, you can build complex retry
// functions that are based off the previous values.
type RetryFunc func(time.Duration) time.Duration

// DefaultRetryFunc is the default return function, which just echos whatever
// duration it was given.
var DefaultRetryFunc RetryFunc = func(t time.Duration) time.Duration {
	return t
}

// dataBufferSize is the default number of views to process in a batch.
const dataBufferSize = 2048

// Watcher is a top-level manager for views that poll Consul for data.
type Watcher struct {
	sync.Mutex

	// DataCh is the chan where Views will be published.
	DataCh chan *View

	// ErrCh is the chan where any errors will be published.
	ErrCh chan error

	// config is the internal configuration of this watcher.
	config *WatcherConfig

	// depViewMap is a map of Templates to Views. Templates are keyed by
	// HashCode().
	depViewMap map[string]*View
}

// WatcherConfig is the configuration for a particular Watcher.
type WatcherConfig struct {
	// Client is the mechanism for communicating with the Consul API.
	Clients *dep.ClientSet

	// Once is used to determine if the views should poll for data exactly once.
	Once bool

	// MaxStale is the maximum staleness of a query. If specified, Consul will
	// distribute work among all servers instead of just the leader. Specifying
	// this option assumes the use of AllowStale.
	MaxStale time.Duration

	// RetryFunc is a RetryFunc that represents the way retrys and backoffs
	// should occur.
	RetryFunc RetryFunc

	// RenewVault determines if the watcher should renew the Vault token as a
	// background job.
	RenewVault bool
}

// NewWatcher creates a new watcher using the given API client.
func NewWatcher(config *WatcherConfig) (*Watcher, error) {
	watcher := &Watcher{config: config}
	if err := watcher.init(); err != nil {
		return nil, err
	}

	return watcher, nil
}

// Add adds the given dependency to the list of monitored depedencies
// and start the associated view. If the dependency already exists, no action is
// taken.
//
// If the Dependency already existed, it this function will return false. If the
// view was successfully created, it will return true. If an error occurs while
// creating the view, it will be returned here (but future errors returned by
// the view will happen on the channel).
func (w *Watcher) Add(d dep.Dependency) (bool, error) {
	w.Lock()
	defer w.Unlock()

	log.Printf("[INFO] (watcher) adding %s", d.Display())

	if _, ok := w.depViewMap[d.HashCode()]; ok {
		log.Printf("[DEBUG] (watcher) %s already exists, skipping", d.Display())
		return false, nil
	}

	v, err := NewView(w.config, d)
	if err != nil {
		return false, err
	}

	log.Printf("[DEBUG] (watcher) %s starting", d.Display())

	w.depViewMap[d.HashCode()] = v
	go v.poll(w.DataCh, w.ErrCh)

	return true, nil
}

// Watching determines if the given dependency is being watched.
func (w *Watcher) Watching(d dep.Dependency) bool {
	w.Lock()
	defer w.Unlock()

	_, ok := w.depViewMap[d.HashCode()]
	return ok
}

// ForceWatching is used to force setting the internal state of watching
// a depedency. This is only used for unit testing purposes.
func (w *Watcher) ForceWatching(d dep.Dependency, enabled bool) {
	w.Lock()
	defer w.Unlock()

	if enabled {
		w.depViewMap[d.HashCode()] = nil
	} else {
		delete(w.depViewMap, d.HashCode())
	}
}

// Remove removes the given dependency from the list and stops the
// associated View. If a View for the given dependency does not exist, this
// function will return false. If the View does exist, this function will return
// true upon successful deletion.
func (w *Watcher) Remove(d dep.Dependency) bool {
	w.Lock()
	defer w.Unlock()

	log.Printf("[INFO] (watcher) removing %s", d.Display())

	if view, ok := w.depViewMap[d.HashCode()]; ok {
		log.Printf("[DEBUG] (watcher) actually removing %s", d.Display())
		view.stop()
		delete(w.depViewMap, d.HashCode())
		return true
	}

	log.Printf("[DEBUG] (watcher) %s did not exist, skipping", d.Display())
	return false
}

// Size returns the number of views this watcher is watching.
func (w *Watcher) Size() int {
	w.Lock()
	defer w.Unlock()
	return len(w.depViewMap)
}

// Stop halts this watcher and any currently polling views immediately. If a
// view was in the middle of a poll, no data will be returned.
func (w *Watcher) Stop() {
	w.Lock()
	defer w.Unlock()

	log.Printf("[INFO] (watcher) stopping all views")

	for _, view := range w.depViewMap {
		if view == nil {
			continue
		}
		log.Printf("[DEBUG] (watcher) stopping %s", view.Dependency.Display())
		view.stop()
	}

	// Reset the map to have no views
	w.depViewMap = make(map[string]*View)

	// Close any idle TCP connections
	w.config.Clients.Stop()
}

// init sets up the initial values for the watcher.
func (w *Watcher) init() error {
	if w.config == nil {
		return fmt.Errorf("watcher: missing config")
	}

	if w.config.RetryFunc == nil {
		w.config.RetryFunc = DefaultRetryFunc
	}

	// Setup the channels
	w.DataCh = make(chan *View, dataBufferSize)
	w.ErrCh = make(chan error)

	// Setup our map of dependencies to views
	w.depViewMap = make(map[string]*View)

	// Start a watcher for the Vault renew if that config was specified
	if w.config.RenewVault {
		vt, err := dep.ParseVaultToken()
		if err != nil {
			return fmt.Errorf("watcher: %s", err)
		}
		if _, err := w.Add(vt); err != nil {
			return fmt.Errorf("watcher: %s", err)
		}
	}

	return nil
}
