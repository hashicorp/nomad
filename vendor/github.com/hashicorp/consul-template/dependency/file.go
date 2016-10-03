package dependency

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

// File represents a local file dependency.
type File struct {
	sync.Mutex
	mutex    sync.RWMutex
	rawKey   string
	lastStat os.FileInfo
	stopped  bool
	stopCh   chan struct{}
}

// Fetch retrieves this dependency and returns the result or any errors that
// occur in the process.
func (d *File) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	d.Lock()
	if d.stopped {
		defer d.Unlock()
		return nil, nil, ErrStopped
	}
	d.Unlock()

	var err error
	var newStat os.FileInfo
	var data []byte

	dataCh := make(chan struct{})
	go func() {
		log.Printf("[DEBUG] (%s) querying file", d.Display())
		newStat, err = d.watch()
		close(dataCh)
	}()

	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	case <-dataCh:
	}

	if err != nil {
		return "", nil, fmt.Errorf("file: error watching: %s", err)
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.lastStat = newStat

	if data, err = ioutil.ReadFile(d.rawKey); err == nil {
		return respWithMetadata(string(data))
	}
	return nil, nil, fmt.Errorf("file: error reading: %s", err)
}

// CanShare returns a boolean if this dependency is shareable.
func (d *File) CanShare() bool {
	return false
}

// HashCode returns a unique identifier.
func (d *File) HashCode() string {
	return fmt.Sprintf("StoreKeyPrefix|%s", d.rawKey)
}

// Display prints the human-friendly output.
func (d *File) Display() string {
	return fmt.Sprintf(`"file(%s)"`, d.rawKey)
}

// Stop halts the dependency's fetch function.
func (d *File) Stop() {
	d.Lock()
	defer d.Unlock()

	if !d.stopped {
		close(d.stopCh)
		d.stopped = true
	}
}

// watch watchers the file for changes
func (d *File) watch() (os.FileInfo, error) {
	for {
		stat, err := os.Stat(d.rawKey)
		if err != nil {
			return nil, err
		}

		changed := func(d *File, stat os.FileInfo) bool {
			d.mutex.RLock()
			defer d.mutex.RUnlock()

			if d.lastStat == nil {
				return true
			}
			if d.lastStat.Size() != stat.Size() {
				return true
			}

			if d.lastStat.ModTime() != stat.ModTime() {
				return true
			}

			return false
		}(d, stat)

		if changed {
			return stat, nil
		}
		time.Sleep(3 * time.Second)
	}
}

// ParseFile creates a file dependency from the given path.
func ParseFile(s string) (*File, error) {
	if len(s) == 0 {
		return nil, errors.New("cannot specify empty file dependency")
	}

	kd := &File{
		rawKey: s,
		stopCh: make(chan struct{}),
	}

	return kd, nil
}
