package monitor

import (
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
)

// Monitor provides a mechanism to stream logs using go-hclog
// InterceptLogger and SinkAdapter. It allows streaming of logs
// at a different log level than what is set on the logger.
type Monitor struct {
	sync.Mutex
	sink            log.SinkAdapter
	logger          log.InterceptLogger
	logCh           chan []byte
	droppedCount    int
	bufSize         int
	droppedDuration time.Duration
}

// New creates a new Monitor. Start must be called in order to actually start
// streaming logs
func New(buf int, logger log.InterceptLogger, opts *log.LoggerOptions) *Monitor {
	sw := &Monitor{
		logger:          logger,
		logCh:           make(chan []byte, buf),
		bufSize:         buf,
		droppedDuration: 3 * time.Second,
	}

	opts.Output = sw
	sink := log.NewSinkAdapter(opts)
	sw.sink = sink

	return sw
}

// Start registers a sink on the monitors logger and starts sending
// received log messages over the returned channel. A non-nil
// sopCh can be used to deregister the sink and stop log streaming
func (d *Monitor) Start(stopCh <-chan struct{}) <-chan []byte {
	d.logger.RegisterSink(d.sink)

	streamCh := make(chan []byte, d.bufSize)
	go func() {
		defer close(streamCh)
		for {
			select {
			case log := <-d.logCh:
				select {
				case <-stopCh:
					d.logger.DeregisterSink(d.sink)
					close(d.logCh)
					return
				case streamCh <- log:
				}
			case <-stopCh:
				d.Lock()
				defer d.Unlock()

				d.logger.DeregisterSink(d.sink)
				close(d.logCh)
				return
			}
		}
	}()

	go func() {
		// loop and check for dropped messages
	LOOP:
		for {
			select {
			case <-stopCh:
				break LOOP
			case <-time.After(d.droppedDuration):
				d.Lock()
				defer d.Unlock()

				if d.droppedCount > 0 {
					dropped := fmt.Sprintf("[WARN] Monitor dropped %d logs during monitor request\n", d.droppedCount)
					select {
					case d.logCh <- []byte(dropped):
					default:
						// Make room for dropped message
						select {
						case <-d.logCh:
							d.droppedCount++
							dropped = fmt.Sprintf("[WARN] Monitor dropped %d logs during monitor request\n", d.droppedCount)
						default:
						}
						d.logCh <- []byte(dropped)
					}
					d.droppedCount = 0
				}
			}
		}
	}()

	return streamCh
}

// Write attempts to send latest log to logCh
// it drops the log if channel is unavailable to receive
func (d *Monitor) Write(p []byte) (n int, err error) {
	bytes := make([]byte, len(p))
	copy(bytes, p)

	select {
	case d.logCh <- bytes:
	default:
		d.droppedCount++
	}
	return
}
