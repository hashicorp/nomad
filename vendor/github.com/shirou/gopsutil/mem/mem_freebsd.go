// +build freebsd

package mem

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func VirtualMemory() (*VirtualMemoryStat, error) {
	return VirtualMemoryWithContext(context.Background())
}

func VirtualMemoryWithContext(ctx context.Context) (*VirtualMemoryStat, error) {
	pageSize, err := unix.SysctlUint32("vm.stats.vm.v_page_size")
	if err != nil {
		return nil, err
	}
	pageCount, err := unix.SysctlUint32("vm.stats.vm.v_page_count")
	if err != nil {
		return nil, err
	}
	free, err := unix.SysctlUint32("vm.stats.vm.v_free_count")
	if err != nil {
		return nil, err
	}
	active, err := unix.SysctlUint32("vm.stats.vm.v_active_count")
	if err != nil {
		return nil, err
	}
	inactive, err := unix.SysctlUint32("vm.stats.vm.v_inactive_count")
	if err != nil {
		return nil, err
	}
	cached, err := unix.SysctlUint32("vm.stats.vm.v_cache_count")
	if err != nil {
		return nil, err
	}
	buffers, err := unix.SysctlUint32("vfs.bufspace")
	if err != nil {
		return nil, err
	}
	wired, err := unix.SysctlUint32("vm.stats.vm.v_wire_count")
	if err != nil {
		return nil, err
	}

	p := uint64(pageSize)
	ret := &VirtualMemoryStat{
		Total:    uint64(pageCount) * p,
		Free:     uint64(free) * p,
		Active:   uint64(active) * p,
		Inactive: uint64(inactive) * p,
		Cached:   uint64(cached) * p,
		Buffers:  uint64(buffers),
		Wired:    uint64(wired) * p,
	}

	ret.Available = ret.Inactive + ret.Cached + ret.Free
	ret.Used = ret.Total - ret.Available
	ret.UsedPercent = float64(ret.Used) / float64(ret.Total) * 100.0

	return ret, nil
}

// Return swapinfo
// FreeBSD can have multiple swap devices. but use only first device
func SwapMemory() (*SwapMemoryStat, error) {
	return SwapMemoryWithContext(context.Background())
}

func SwapMemoryWithContext(ctx context.Context) (*SwapMemoryStat, error) {
	swapinfo, err := exec.LookPath("swapinfo")
	if err != nil {
		return nil, err
	}

	out, err := invoke.Command(swapinfo)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		values := strings.Fields(line)
		// skip title line
		if len(values) == 0 || values[0] == "Device" {
			continue
		}

		u := strings.Replace(values[4], "%", "", 1)
		total_v, err := strconv.ParseUint(values[1], 10, 64)
		if err != nil {
			return nil, err
		}
		used_v, err := strconv.ParseUint(values[2], 10, 64)
		if err != nil {
			return nil, err
		}
		free_v, err := strconv.ParseUint(values[3], 10, 64)
		if err != nil {
			return nil, err
		}
		up_v, err := strconv.ParseFloat(u, 64)
		if err != nil {
			return nil, err
		}

		return &SwapMemoryStat{
			Total:       total_v,
			Used:        used_v,
			Free:        free_v,
			UsedPercent: up_v,
		}, nil
	}

	return nil, errors.New("no swap devices found")
}
