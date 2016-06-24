package driver

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/config"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

type FakeDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

type FakeDriverConfig struct {
	Success float64 `mapstructure:"success"`
	Failure float64 `mapstructure:"failure"`
	Crash   float64 `mapstructure:"failure"`
}

type fakeDriverHandle struct {
	logger *log.Logger
	waitCh chan *dstructs.WaitResult
	doneCh chan struct{}
}

func NewFakeDriver(ctx *DriverContext) Driver {
	return &FakeDriver{DriverContext: *ctx}
}

func (d *FakeDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	return false, nil
}

func (d *FakeDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"success": &fields.FieldSchema{
				Type:     fields.TypeFloat,
				Required: true,
			},
			"failure": &fields.FieldSchema{
				Type: fields.TypeFloat,
			},
			"crash": &fields.FieldSchema{
				Type: fields.TypeFloat,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
	}

	return nil
}

func (d *FakeDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig FakeDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}

	total := driverConfig.Success + driverConfig.Crash + driverConfig.Failure
	if total != 100 {
		return nil, fmt.Errorf("sum of success, crash and failure has to be 1, got: %v", total)
	}

	h := &fakeDriverHandle{
		logger: d.logger,
		doneCh: make(chan struct{}),
		waitCh: make(chan *dstructs.WaitResult, 1),
	}

	return h, nil
}

func (d *FakeDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &fakeDriverHandle{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle %q: %v", handleID, err)
	}
	return nil, nil
}

func (h *fakeDriverHandle) ID() string {
	id := fakeDriverHandle{}
	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.fake: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *fakeDriverHandle) Kill() error {
	return nil
}

func (h *fakeDriverHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return nil, nil
}

func (h *fakeDriverHandle) Update(task *structs.Task) error {
	return nil
}

func (h *fakeDriverHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}
