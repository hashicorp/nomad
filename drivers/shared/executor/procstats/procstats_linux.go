package procstats

import (
	"maps"
	"sync"
	"time"

	// "github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
	"oss.indeed.com/go/libtime"
)

func New(pl ProcessList) ProcessStats {
	const cacheTTL = 5 * time.Second
	return &linuxProcStats{
		procList: pl,
		cacheTTL: cacheTTL,
		clock:    libtime.SystemClock(),
	}
}

type linuxProcStats struct {
	cacheTTL time.Duration
	procList ProcessList
	clock    libtime.Clock

	lock   sync.Mutex
	latest ProcUsages
	at     time.Time
}

func (lps *linuxProcStats) expired() bool {
	age := lps.clock.Since(lps.at)
	return age > lps.cacheTTL
}

func (lps *linuxProcStats) StatProcesses() ProcUsages {
	lps.lock.Lock()
	defer lps.lock.Unlock()

	if !lps.expired() {
		return maps.Clone(lps.latest)
	}

	lps.latest = make(ProcUsages)

	procs := lps.procList.ListProcesses()
	_ = procs
	// do something with procs

	return maps.Clone(lps.latest)
}
