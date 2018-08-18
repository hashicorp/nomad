package fingerprint

import (
	"bytes"
	"fmt"
	"log"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mindprince/gonvml"
)

type NvidiaGPUFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

// TODO(oleksii-shyman): find better place for interfaces declaration and implementation

type NVMLDevice interface {
	UUID() (string, error)
	Name() (string, error)
	MemoryInfo() (uint64, uint64, error)
}

type NVMLDriver interface {
	Initialize() error
	Shutdown() error
	SystemDriverVersion() (string, error)
	DeviceCount() (uint, error)
	DeviceHandleByIndex(uint) (NVMLDevice, error)
}

type NVMLDriverImplementation struct{}

func (n *NVMLDriverImplementation) Initialize() error {
	return gonvml.Initialize()
}

func (n *NVMLDriverImplementation) Shutdown() error {
	return gonvml.Shutdown()
}

func (n *NVMLDriverImplementation) SystemDriverVersion() (string, error) {
	return gonvml.SystemDriverVersion()
}

func (n *NVMLDriverImplementation) DeviceCount() (uint, error) {
	return gonvml.DeviceCount()
}

func (n *NVMLDriverImplementation) DeviceHandleByIndex(index uint) (NVMLDevice, error) {
	return gonvml.DeviceHandleByIndex(index)
}

func getDataFromNVML(driver NVMLDriver) (error, bool, []*structs.NvidiaGPUResource) {
	/*
		nvml fields to be fingerprinted # nvml_library_call
		1 - Driver Version              # nvmlSystemGetDriverVersion
		2 - Product Name                # nvmlDeviceGetName
		3 - GPU UUID                    # nvmlDeviceGetUUID
		4 - Total Memory                # nvmlDeviceGetMemoryInfo
	*/
	err := driver.Initialize()
	if err != nil {
		// There was an error during initialization, this node would not report
		// any functioning GPUs
		return err, false, nil
	}
	defer driver.Shutdown()

	driverVersion, err := driver.SystemDriverVersion()
	if err != nil {
		return fmt.Errorf("nvidia nvml SystemDriverVersion() error: %v\n", err), true, nil
	}

	numDevices, err := driver.DeviceCount()
	if err != nil {
		return fmt.Errorf("nvidia nvml DeviceCount() error: %v\n", err), true, nil
	}

	allNvidiaGPUResources := make([]*structs.NvidiaGPUResource, numDevices)

	for i := 0; i < int(numDevices); i++ {
		dev, err := driver.DeviceHandleByIndex(uint(i))
		if err != nil {
			return fmt.Errorf("nvidia nvml DeviceHandleByIndex() error: %v\n", err), true, nil
		}

		uuid, err := dev.UUID()
		if err != nil {
			return fmt.Errorf("nvidia nvml dev.UUID() error: %v\n", err), true, nil
		}

		deviceName, err := dev.Name()
		if err != nil {
			return fmt.Errorf("nvidia nvml dev.Name() error: %v\n", err), true, nil
		}

		totalMemory, _, err := dev.MemoryInfo()
		if err != nil {
			return fmt.Errorf("nvidia nvml dev.MemoryInfo() error: %v\n", err), true, nil
		}

		allNvidiaGPUResources[i] = &structs.NvidiaGPUResource{
			DriverVersion: driverVersion,
			ModelName:     deviceName,
			UUID:          uuid,
			// totalMemory returns amount in bytes
			// to convert in mebibytes -> we need to divide it to 2**20
			MemoryMiB: totalMemory / 1024 / 1024,
		}
	}
	return nil, false, allNvidiaGPUResources
}

func NewNvidiaGPUFingerprint(logger *log.Logger) Fingerprint {
	return &NvidiaGPUFingerprint{logger: logger}
}

func (f *NvidiaGPUFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	/*
		Config section:
			There is a possible situation, when node has nvidia gpu, but
			cluster operator does not want to attach those gpu resources to
			nomad workload. Config section would consist of one entry ->
			"restrict_nvidia". If restrict_nvidia set to true -> no resources
			would be fingerprinted
	*/
	cfg := req.Config
	if cfg.RestrictNvidia {
		f.logger.Printf("[DEBUG] operator restricted NVIDIA GPUs usage")
		resp.Detected = false
		return nil
	}
	err, shouldTerminate, allNvidiaGPUResources := getDataFromNVML(&NVMLDriverImplementation{})
	if err != nil {
		f.logger.Printf("[DEBUG] failed to get data from NVML driver with error '%v'", err)
		if shouldTerminate {
			return err
		}
		resp.Detected = false
	} else {
		resp.AddAttribute("nvidia.totalgpus", fmt.Sprintf("%d", len(allNvidiaGPUResources)))
		resp.Detected = true
		resp.Resources = &structs.Resources{
			NvidiaGPUResources: allNvidiaGPUResources,
		}
		// log all resources fingerprinted
		var logBuffer bytes.Buffer
		for index, element := range allNvidiaGPUResources {
			if index != 0 {
				logBuffer.WriteString(", ")
			}
			logBuffer.WriteString(element.GoString())
		}
		f.logger.Printf("[DEBUG] nvidia.gpu: registered following gpus '%s'", logBuffer.String())
	}

	return nil
}
