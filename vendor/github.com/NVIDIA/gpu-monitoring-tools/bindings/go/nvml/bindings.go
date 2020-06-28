/*
 * Copyright (c) 2020, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package nvml

/*
#cgo linux LDFLAGS: -ldl -Wl,--unresolved-symbols=ignore-in-object-files
#cgo darwin LDFLAGS: -ldl -Wl,-undefined,dynamic_lookup
#cgo windows LDFLAGS: -LC:/Program\ Files/NVIDIA\ Corporation/NVSMI -lnvml
#include "nvml.h"

#undef nvmlEventSetWait
nvmlReturn_t DECLDIR nvmlEventSetWait(nvmlEventSet_t set, nvmlEventData_t * data, unsigned int timeoutms);
nvmlReturn_t DECLDIR nvmlEventSetWait_v2(nvmlEventSet_t set, nvmlEventData_t * data, unsigned int timeoutms);
*/
import "C"

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	szDriver   = C.NVML_SYSTEM_DRIVER_VERSION_BUFFER_SIZE
	szName     = C.NVML_DEVICE_NAME_BUFFER_SIZE
	szUUID     = C.NVML_DEVICE_UUID_V2_BUFFER_SIZE
	szProcs    = 32
	szProcName = 64

	XidCriticalError = C.nvmlEventTypeXidCriticalError
)

type handle struct{ dev C.nvmlDevice_t }
type EventSet struct{ set C.nvmlEventSet_t }
type Event struct {
	UUID              *string
	GpuInstanceId     *uint
	ComputeInstanceId *uint
	Etype             uint64
	Edata             uint64
}

func uintPtr(c C.uint) *uint {
	i := uint(c)
	return &i
}

func uint64Ptr(c C.ulonglong) *uint64 {
	i := uint64(c)
	return &i
}

func stringPtr(c *C.char) *string {
	s := C.GoString(c)
	return &s
}

func errorString(ret C.nvmlReturn_t) error {
	if ret == C.NVML_SUCCESS {
		return nil
	}
	err := C.GoString(C.nvmlErrorString(ret))
	return fmt.Errorf("nvml: %v", err)
}

func init_() error {
	r := dl.nvmlInit()
	if r == C.NVML_ERROR_LIBRARY_NOT_FOUND {
		return errors.New("could not load NVML library")
	}
	return errorString(r)
}

func NewEventSet() EventSet {
	var set C.nvmlEventSet_t
	C.nvmlEventSetCreate(&set)

	return EventSet{set}
}

func RegisterEvent(es EventSet, event int) error {
	n, err := deviceGetCount()
	if err != nil {
		return err
	}

	var i uint
	for i = 0; i < n; i++ {
		h, err := deviceGetHandleByIndex(i)
		if err != nil {
			return err
		}

		r := C.nvmlDeviceRegisterEvents(h.dev, C.ulonglong(event), es.set)
		if r != C.NVML_SUCCESS {
			return errorString(r)
		}
	}

	return nil
}

func RegisterEventForDevice(es EventSet, event int, uuid string) error {
	n, err := deviceGetCount()
	if err != nil {
		return err
	}

	var i uint
	for i = 0; i < n; i++ {
		h, err := deviceGetHandleByIndex(i)
		if err != nil {
			return err
		}

		duuid, err := h.deviceGetUUID()
		if err != nil {
			return err
		}

		if *duuid != uuid {
			continue
		}

		r := C.nvmlDeviceRegisterEvents(h.dev, C.ulonglong(event), es.set)
		if r != C.NVML_SUCCESS {
			return errorString(r)
		}

		return nil
	}

	return fmt.Errorf("nvml: device not found")
}

func DeleteEventSet(es EventSet) {
	C.nvmlEventSetFree(es.set)
}

func WaitForEvent(es EventSet, timeout uint) (Event, error) {
	var data C.nvmlEventData_t

	r := dl.lookupSymbol("nvmlEventSetWait_v2")
	if r == C.NVML_SUCCESS {
		r = C.nvmlEventSetWait_v2(es.set, &data, C.uint(timeout))
	} else {
		r = C.nvmlEventSetWait(es.set, &data, C.uint(timeout))
		data.gpuInstanceId = 0xFFFFFFFF
		data.computeInstanceId = 0xFFFFFFFF
	}
	if r != C.NVML_SUCCESS {
		return Event{}, errorString(r)
	}

	uuid, _ := handle{data.device}.deviceGetUUID()

	return Event{
		UUID:              uuid,
		Etype:             uint64(data.eventType),
		Edata:             uint64(data.eventData),
		GpuInstanceId:     uintPtr(data.gpuInstanceId),
		ComputeInstanceId: uintPtr(data.computeInstanceId),
	}, nil
}

func shutdown() error {
	return errorString(dl.nvmlShutdown())
}

func systemGetCudaDriverVersion() (*uint, *uint, error) {
	var v C.int

	r := C.nvmlSystemGetCudaDriverVersion_v2(&v)
	if r != C.NVML_SUCCESS {
		return nil, nil, errorString(r)
	}

	major := uint(v / 1000)
	minor := uint(v % 1000 / 10)

	return &major, &minor, errorString(r)
}

func systemGetDriverVersion() (string, error) {
	var driver [szDriver]C.char

	r := C.nvmlSystemGetDriverVersion(&driver[0], szDriver)
	return C.GoString(&driver[0]), errorString(r)
}

func systemGetProcessName(pid uint) (string, error) {
	var proc [szProcName]C.char

	r := C.nvmlSystemGetProcessName(C.uint(pid), &proc[0], szProcName)
	return C.GoString(&proc[0]), errorString(r)
}

func deviceGetCount() (uint, error) {
	var n C.uint

	r := C.nvmlDeviceGetCount(&n)
	return uint(n), errorString(r)
}

func deviceGetHandleByIndex(idx uint) (handle, error) {
	var dev C.nvmlDevice_t

	r := C.nvmlDeviceGetHandleByIndex(C.uint(idx), &dev)
	return handle{dev}, errorString(r)
}

func deviceGetTopologyCommonAncestor(h1, h2 handle) (*uint, error) {
	r := dl.lookupSymbol("nvmlDeviceGetTopologyCommonAncestor")
	if r == C.NVML_ERROR_FUNCTION_NOT_FOUND {
		return nil, nil
	}

	var level C.nvmlGpuTopologyLevel_t
	r = C.nvmlDeviceGetTopologyCommonAncestor(h1.dev, h2.dev, &level)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}

	return uintPtr(C.uint(level)), errorString(r)
}

func (h handle) deviceGetCudaComputeCapability() (*int, *int, error) {
	var major, minor C.int

	r := C.nvmlDeviceGetCudaComputeCapability(h.dev, &major, &minor)
	if r != C.NVML_SUCCESS {
		return nil, nil, errorString(r)
	}

	intMajor := int(major)
	intMinor := int(minor)

	return &intMajor, &intMinor, errorString(r)
}

func (h handle) deviceGetName() (*string, error) {
	var name [szName]C.char

	r := C.nvmlDeviceGetName(h.dev, &name[0], szName)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return stringPtr(&name[0]), errorString(r)
}

func (h handle) deviceGetIndex() (*uint, error) {
	var index C.uint
	r := C.nvmlDeviceGetIndex(h.dev, &index)
	if r != C.NVML_SUCCESS {
		return nil, errorString(r)
	}
	return uintPtr(index), nil
}

func (h handle) deviceGetUUID() (*string, error) {
	var uuid [szUUID]C.char

	r := C.nvmlDeviceGetUUID(h.dev, &uuid[0], szUUID)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return stringPtr(&uuid[0]), errorString(r)
}

func (h handle) deviceGetPciInfo() (*string, error) {
	var pci C.nvmlPciInfo_t

	r := C.nvmlDeviceGetPciInfo(h.dev, &pci)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return stringPtr(&pci.busId[0]), errorString(r)
}

func (h handle) deviceGetMinorNumber() (*uint, error) {
	var minor C.uint

	r := C.nvmlDeviceGetMinorNumber(h.dev, &minor)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(minor), errorString(r)
}

func (h handle) deviceGetBAR1MemoryInfo() (*uint64, *uint64, error) {
	var bar1 C.nvmlBAR1Memory_t

	r := C.nvmlDeviceGetBAR1MemoryInfo(h.dev, &bar1)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil
	}
	return uint64Ptr(bar1.bar1Total), uint64Ptr(bar1.bar1Used), errorString(r)
}

func (h handle) deviceGetNvLinkState(link uint) (*uint, error) {
	var isActive C.nvmlEnableState_t

	r := C.nvmlDeviceGetNvLinkState(h.dev, C.uint(link), &isActive)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}

	return uintPtr(C.uint(isActive)), errorString(r)
}

func (h handle) deviceGetNvLinkRemotePciInfo(link uint) (*string, error) {
	var pci C.nvmlPciInfo_t

	r := C.nvmlDeviceGetNvLinkRemotePciInfo(h.dev, C.uint(link), &pci)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}

	return stringPtr(&pci.busId[0]), errorString(r)
}

func (h handle) deviceGetAllNvLinkRemotePciInfo() ([]*string, error) {
	busIds := []*string{}

	for i := uint(0); i < C.NVML_NVLINK_MAX_LINKS; i++ {
		state, err := h.deviceGetNvLinkState(i)
		if err != nil {
			return nil, err
		}

		if state == nil {
			continue
		}

		if *state == C.NVML_FEATURE_ENABLED {
			pci, err := h.deviceGetNvLinkRemotePciInfo(i)
			if err != nil {
				return nil, err
			}

			if pci == nil {
				continue
			}

			busIds = append(busIds, pci)
		}
	}

	return busIds, nil
}

func (h handle) deviceGetPowerManagementLimit() (*uint, error) {
	var power C.uint

	r := C.nvmlDeviceGetPowerManagementLimit(h.dev, &power)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(power), errorString(r)
}

func (h handle) deviceGetMaxClockInfo() (*uint, *uint, error) {
	var sm, mem C.uint

	r := C.nvmlDeviceGetMaxClockInfo(h.dev, C.NVML_CLOCK_SM, &sm)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil
	}
	if r == C.NVML_SUCCESS {
		r = C.nvmlDeviceGetMaxClockInfo(h.dev, C.NVML_CLOCK_MEM, &mem)
	}
	return uintPtr(sm), uintPtr(mem), errorString(r)
}

func (h handle) deviceGetMaxPcieLinkGeneration() (*uint, error) {
	var link C.uint

	r := C.nvmlDeviceGetMaxPcieLinkGeneration(h.dev, &link)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(link), errorString(r)
}

func (h handle) deviceGetMaxPcieLinkWidth() (*uint, error) {
	var width C.uint

	r := C.nvmlDeviceGetMaxPcieLinkWidth(h.dev, &width)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(width), errorString(r)
}

func (h handle) deviceGetPowerUsage() (*uint, error) {
	var power C.uint

	r := C.nvmlDeviceGetPowerUsage(h.dev, &power)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(power), errorString(r)
}

func (h handle) deviceGetFanSpeed() (*uint, error) {
	var speed C.uint

	r := C.nvmlDeviceGetFanSpeed(h.dev, &speed)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(speed), errorString(r)
}

func (h handle) deviceGetTemperature() (*uint, error) {
	var temp C.uint

	r := C.nvmlDeviceGetTemperature(h.dev, C.NVML_TEMPERATURE_GPU, &temp)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(temp), errorString(r)
}

func (h handle) deviceGetUtilizationRates() (*uint, *uint, error) {
	var usage C.nvmlUtilization_t

	r := C.nvmlDeviceGetUtilizationRates(h.dev, &usage)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil
	}
	return uintPtr(usage.gpu), uintPtr(usage.memory), errorString(r)
}

func (h handle) deviceGetEncoderUtilization() (*uint, error) {
	var usage, sampling C.uint

	r := C.nvmlDeviceGetEncoderUtilization(h.dev, &usage, &sampling)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(usage), errorString(r)
}

func (h handle) deviceGetDecoderUtilization() (*uint, error) {
	var usage, sampling C.uint

	r := C.nvmlDeviceGetDecoderUtilization(h.dev, &usage, &sampling)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil
	}
	return uintPtr(usage), errorString(r)
}

func (h handle) deviceGetMemoryInfo() (totalMem *uint64, devMem DeviceMemory, err error) {
	var mem C.nvmlMemory_t

	r := C.nvmlDeviceGetMemoryInfo(h.dev, &mem)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return
	}

	err = errorString(r)
	if r != C.NVML_SUCCESS {
		return
	}

	totalMem = uint64Ptr(mem.total)
	if totalMem != nil {
		*totalMem /= 1024 * 1024 // MiB
	}

	devMem = DeviceMemory{
		Used: uint64Ptr(mem.used),
		Free: uint64Ptr(mem.free),
	}

	if devMem.Used != nil {
		*devMem.Used /= 1024 * 1024 // MiB
	}

	if devMem.Free != nil {
		*devMem.Free /= 1024 * 1024 // MiB
	}
	return
}

func (h handle) deviceGetClockInfo() (*uint, *uint, error) {
	var sm, mem C.uint

	r := C.nvmlDeviceGetClockInfo(h.dev, C.NVML_CLOCK_SM, &sm)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil
	}
	if r == C.NVML_SUCCESS {
		r = C.nvmlDeviceGetClockInfo(h.dev, C.NVML_CLOCK_MEM, &mem)
	}
	return uintPtr(sm), uintPtr(mem), errorString(r)
}

func (h handle) deviceGetMemoryErrorCounter() (*uint64, *uint64, *uint64, error) {
	var l1, l2, mem C.ulonglong

	r := C.nvmlDeviceGetMemoryErrorCounter(h.dev, C.NVML_MEMORY_ERROR_TYPE_UNCORRECTED,
		C.NVML_VOLATILE_ECC, C.NVML_MEMORY_LOCATION_L1_CACHE, &l1)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil, nil
	}
	if r == C.NVML_SUCCESS {
		r = C.nvmlDeviceGetMemoryErrorCounter(h.dev, C.NVML_MEMORY_ERROR_TYPE_UNCORRECTED,
			C.NVML_VOLATILE_ECC, C.NVML_MEMORY_LOCATION_L2_CACHE, &l2)
	}
	if r == C.NVML_SUCCESS {
		r = C.nvmlDeviceGetMemoryErrorCounter(h.dev, C.NVML_MEMORY_ERROR_TYPE_UNCORRECTED,
			C.NVML_VOLATILE_ECC, C.NVML_MEMORY_LOCATION_DEVICE_MEMORY, &mem)
	}
	return uint64Ptr(l1), uint64Ptr(l2), uint64Ptr(mem), errorString(r)
}

func (h handle) deviceGetPcieThroughput() (*uint, *uint, error) {
	var rx, tx C.uint

	r := C.nvmlDeviceGetPcieThroughput(h.dev, C.NVML_PCIE_UTIL_RX_BYTES, &rx)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil
	}
	if r == C.NVML_SUCCESS {
		r = C.nvmlDeviceGetPcieThroughput(h.dev, C.NVML_PCIE_UTIL_TX_BYTES, &tx)
	}
	return uintPtr(rx), uintPtr(tx), errorString(r)
}

func (h handle) deviceGetComputeRunningProcesses() ([]uint, []uint64, error) {
	var procs [szProcs]C.nvmlProcessInfo_t
	var count = C.uint(szProcs)

	r := C.nvmlDeviceGetComputeRunningProcesses(h.dev, &count, &procs[0])
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil
	}
	n := int(count)
	pids := make([]uint, n)
	mems := make([]uint64, n)
	for i := 0; i < n; i++ {
		pids[i] = uint(procs[i].pid)
		mems[i] = uint64(procs[i].usedGpuMemory)
	}
	return pids, mems, errorString(r)
}

func (h handle) deviceGetGraphicsRunningProcesses() ([]uint, []uint64, error) {
	var procs [szProcs]C.nvmlProcessInfo_t
	var count = C.uint(szProcs)

	r := C.nvmlDeviceGetGraphicsRunningProcesses(h.dev, &count, &procs[0])
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return nil, nil, nil
	}
	n := int(count)
	pids := make([]uint, n)
	mems := make([]uint64, n)
	for i := 0; i < n; i++ {
		pids[i] = uint(procs[i].pid)
		mems[i] = uint64(procs[i].usedGpuMemory)
	}
	return pids, mems, errorString(r)
}

func (h handle) deviceGetAllRunningProcesses() ([]ProcessInfo, error) {
	cPids, cpMems, err := h.deviceGetComputeRunningProcesses()
	if err != nil {
		return nil, err
	}

	gPids, gpMems, err := h.deviceGetGraphicsRunningProcesses()
	if err != nil {
		return nil, err
	}

	allPids := make(map[uint]ProcessInfo)

	for i, pid := range cPids {
		name, err := processName(pid)
		if err != nil {
			return nil, err
		}
		allPids[pid] = ProcessInfo{
			PID:        pid,
			Name:       name,
			MemoryUsed: cpMems[i] / (1024 * 1024), // MiB
			Type:       Compute,
		}

	}

	for i, pid := range gPids {
		pInfo, exists := allPids[pid]
		if exists {
			pInfo.Type = ComputeAndGraphics
			allPids[pid] = pInfo
		} else {
			name, err := processName(pid)
			if err != nil {
				return nil, err
			}
			allPids[pid] = ProcessInfo{
				PID:        pid,
				Name:       name,
				MemoryUsed: gpMems[i] / (1024 * 1024), // MiB
				Type:       Graphics,
			}
		}
	}

	var processInfo []ProcessInfo
	for _, v := range allPids {
		processInfo = append(processInfo, v)
	}
	sort.Slice(processInfo, func(i, j int) bool {
		return processInfo[i].PID < processInfo[j].PID
	})

	return processInfo, nil
}

func (h handle) getClocksThrottleReasons() (reason ThrottleReason, err error) {
	var clocksThrottleReasons C.ulonglong

	r := C.nvmlDeviceGetCurrentClocksThrottleReasons(h.dev, &clocksThrottleReasons)

	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return ThrottleReasonUnknown, nil
	}

	if r != C.NVML_SUCCESS {
		return ThrottleReasonUnknown, errorString(r)
	}

	switch clocksThrottleReasons {
	case C.nvmlClocksThrottleReasonGpuIdle:
		reason = ThrottleReasonGpuIdle
	case C.nvmlClocksThrottleReasonApplicationsClocksSetting:
		reason = ThrottleReasonApplicationsClocksSetting
	case C.nvmlClocksThrottleReasonSwPowerCap:
		reason = ThrottleReasonSwPowerCap
	case C.nvmlClocksThrottleReasonHwSlowdown:
		reason = ThrottleReasonHwSlowdown
	case C.nvmlClocksThrottleReasonSyncBoost:
		reason = ThrottleReasonSyncBoost
	case C.nvmlClocksThrottleReasonSwThermalSlowdown:
		reason = ThrottleReasonSwThermalSlowdown
	case C.nvmlClocksThrottleReasonHwThermalSlowdown:
		reason = ThrottleReasonHwThermalSlowdown
	case C.nvmlClocksThrottleReasonHwPowerBrakeSlowdown:
		reason = ThrottleReasonHwPowerBrakeSlowdown
	case C.nvmlClocksThrottleReasonDisplayClockSetting:
		reason = ThrottleReasonDisplayClockSetting
	case C.nvmlClocksThrottleReasonNone:
		reason = ThrottleReasonNone
	}
	return
}

func (h handle) getPerformanceState() (PerfState, error) {
	var pstate C.nvmlPstates_t

	r := C.nvmlDeviceGetPerformanceState(h.dev, &pstate)

	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return PerfStateUnknown, nil
	}

	if r != C.NVML_SUCCESS {
		return PerfStateUnknown, errorString(r)
	}
	return PerfState(pstate), nil
}

func processName(pid uint) (string, error) {
	f := `/proc/` + strconv.FormatUint(uint64(pid), 10) + `/comm`
	d, err := ioutil.ReadFile(f)

	if err != nil {
		// TOCTOU: process terminated
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSuffix(string(d), "\n"), err
}

func (h handle) getAccountingInfo() (accountingInfo Accounting, err error) {
	var mode C.nvmlEnableState_t
	var buffer C.uint

	r := C.nvmlDeviceGetAccountingMode(h.dev, &mode)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return
	}

	if r != C.NVML_SUCCESS {
		return accountingInfo, errorString(r)
	}

	r = C.nvmlDeviceGetAccountingBufferSize(h.dev, &buffer)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return
	}

	if r != C.NVML_SUCCESS {
		return accountingInfo, errorString(r)
	}

	accountingInfo = Accounting{
		Mode:       ModeState(mode),
		BufferSize: uintPtr(buffer),
	}
	return
}

func (h handle) getDisplayInfo() (display Display, err error) {
	var mode, isActive C.nvmlEnableState_t

	r := C.nvmlDeviceGetDisplayActive(h.dev, &mode)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return
	}

	if r != C.NVML_SUCCESS {
		return display, errorString(r)
	}

	r = C.nvmlDeviceGetDisplayMode(h.dev, &isActive)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return
	}
	if r != C.NVML_SUCCESS {
		return display, errorString(r)
	}
	display = Display{
		Mode:   ModeState(mode),
		Active: ModeState(isActive),
	}
	return
}

func (h handle) getPeristenceMode() (state ModeState, err error) {
	var mode C.nvmlEnableState_t

	r := C.nvmlDeviceGetPersistenceMode(h.dev, &mode)
	if r == C.NVML_ERROR_NOT_SUPPORTED {
		return
	}
	return ModeState(mode), errorString(r)
}

func (h *handle) isMigEnabled() (bool, error) {
	ret := dl.lookupSymbol("nvmlDeviceGetMigMode")
	if ret != C.NVML_SUCCESS {
		return false, nil
	}

	var cm, pm C.uint
	ret = C.nvmlDeviceGetMigMode(h.dev, &cm, &pm)
	if ret == C.NVML_ERROR_NOT_SUPPORTED {
		return false, nil
	}
	if ret != C.NVML_SUCCESS {
		return false, errorString(ret)
	}

	return (cm == C.NVML_DEVICE_MIG_ENABLE) && (cm == pm), nil
}

func (h *handle) getMigDevices() ([]handle, error) {
	ret := dl.lookupSymbol("nvmlDeviceGetMaxMigDeviceCount")
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	var c C.uint
	ret = C.nvmlDeviceGetMaxMigDeviceCount(h.dev, &c)
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	ret = dl.lookupSymbol("nvmlDeviceGetMigDeviceHandleByIndex")
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	var handles []handle
	for i := 0; i < int(c); i++ {
		var mig C.nvmlDevice_t
		ret := C.nvmlDeviceGetMigDeviceHandleByIndex(h.dev, C.uint(i), &mig)
		if ret == C.NVML_ERROR_NOT_FOUND {
			continue
		}
		if ret != C.NVML_SUCCESS {
			return nil, errorString(ret)
		}

		handles = append(handles, handle{mig})
	}

	return handles, nil
}

func (h *handle) deviceGetDeviceHandleFromMigDeviceHandle() (handle, error) {
	ret := dl.lookupSymbol("nvmlDeviceGetDeviceHandleFromMigDeviceHandle")
	if ret != C.NVML_SUCCESS {
		return handle{}, errorString(ret)
	}

	var parent C.nvmlDevice_t
	ret = C.nvmlDeviceGetDeviceHandleFromMigDeviceHandle(h.dev, &parent)
	if ret != C.NVML_SUCCESS {
		return handle{}, errorString(ret)
	}

	return handle{parent}, nil
}
