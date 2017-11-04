package fingerprint

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

type NvidiaGPUFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

func NewNvidiaGPUFingerprint(logger *log.Logger) Fingerprint {
	return &NvidiaGPUFingerprint{logger: logger}
}

func (f *NvidiaGPUFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	nvidiaSmiPath, err := exec.LookPath("nvidia-smi")
	if err != nil {
		f.logger.Printf("[ERR] fingerprint.nvidia: error looking up nvidia smi: %v", err)
		return false, nil
	}
	output, err := exec.Command(nvidiaSmiPath, "--query-gpu=index,driver_version,gpu_name,uuid,memory.total", "--format=csv").Output()
	if err != nil {
		f.logger.Printf("[ERR] fingerprint.nvidia: error executing nvidia smi: %v", err)
		return false, nil
	}
	gpus, err := f.parseOutput(output)
	if err != nil {
		f.logger.Printf("[ERR] fingerprint.nvidia: error parsing nvidia smi output: %v", err)
		return false, nil
	}
	node.Attributes["gpus.total"] = fmt.Sprintf("%d", len(gpus))
	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}
	node.Resources.NvidiaGPUResources = gpus

	return true, nil
}

func (f *NvidiaGPUFingerprint) parseOutput(output []byte) ([]*structs.NvidiaGPUResource, error) {
	gpus := make([]*structs.NvidiaGPUResource, 0)
	r := bufio.NewReader(bytes.NewReader(output))

	// Discard the first line since it's an header
	r.ReadString('\n')

	// Scan each line and construct a gpu resource
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		tokens := strings.Split(line, ",")
		if len(tokens) != 5 {
			return nil, fmt.Errorf("malformed output: %v", line)
		}
		index, err := strconv.Atoi(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("malformed output: %v: %v", err, line)
		}
		mem := strings.TrimSuffix(strings.TrimSpace(tokens[4]), "MiB")
		memoryMB, err := strconv.Atoi(strings.TrimSpace(mem))
		if err != nil {
			return nil, fmt.Errorf("malformed output: %v: %v", err, tokens[4])
		}
		gpu := &structs.NvidiaGPUResource{
			Index:         index,
			DriverVersion: tokens[1],
			ModelName:     tokens[2],
			UUID:          tokens[3],
			MemoryMB:      memoryMB,
		}
		gpus = append(gpus, gpu)
	}
	return gpus, nil
}
