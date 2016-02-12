// +build linux freebsd darwin

package cpu

import (
	"fmt"
	"time"
)

func CPUPercent(interval time.Duration, percpu bool) ([]float64, error) {
	getAllBusy := func(t CPUTimesStat) (float64, float64) {
		busy := t.User + t.System + t.Nice + t.Iowait + t.Irq +
			t.Softirq + t.Steal + t.Guest + t.GuestNice + t.Stolen
		return busy + t.Idle, busy
	}

	calculate := func(t1, t2 CPUTimesStat) float64 {
		t1All, t1Busy := getAllBusy(t1)
		t2All, t2Busy := getAllBusy(t2)

		if t2Busy <= t1Busy {
			return 0
		}
		if t2All <= t1All {
			return 1
		}
		return (t2Busy - t1Busy) / (t2All - t1All) * 100
	}

	// Get CPU usage at the start of the interval.
	cpuTimes1, err := CPUTimes(percpu)
	if err != nil {
		return nil, err
	}

	if interval > 0 {
		time.Sleep(interval)
	}

	// And at the end of the interval.
	cpuTimes2, err := CPUTimes(percpu)
	if err != nil {
		return nil, err
	}

	// Make sure the CPU measurements have the same length.
	if len(cpuTimes1) != len(cpuTimes2) {
		return nil, fmt.Errorf(
			"received two CPU counts: %d != %d",
			len(cpuTimes1), len(cpuTimes2),
		)
	}

	ret := make([]float64, len(cpuTimes1))
	for i, t := range cpuTimes2 {
		ret[i] = calculate(cpuTimes1[i], t)
	}
	return ret, nil
}
