package command

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	hcl2 "github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcldec"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/kr/pretty"
	"github.com/mitchellh/cli"
	"github.com/zclconf/go-cty/cty/msgpack"
)

func DeviceCommandFactory(meta Meta) cli.CommandFactory {
	return func() (cli.Command, error) {
		return &Device{Meta: meta}, nil
	}
}

type Device struct {
	Meta

	// dev is the plugin device
	dev device.DevicePlugin

	// spec is the returned and parsed spec.
	spec hcldec.Spec
}

func (c *Device) Help() string {
	helpText := `
Usage: nomad-plugin-launcher device <device-binary> <config_file>

  Device launches the given device binary and provides a REPL for interacting
  with it.

General Options:

` + generalOptionsUsage() + `

Device Options:
  
  -trace
    Enable trace level log output.
`

	return strings.TrimSpace(helpText)
}

func (c *Device) Synopsis() string {
	return "REPL for interacting with device plugins"
}

func (c *Device) Run(args []string) int {
	var trace bool
	cmdFlags := c.FlagSet("device")
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&trace, "trace", false, "")

	if err := cmdFlags.Parse(args); err != nil {
		c.logger.Error("failed to parse flags:", "error", err)
		return 1
	}
	if trace {
		c.logger.SetLevel(hclog.Trace)
	} else if c.verbose {
		c.logger.SetLevel(hclog.Debug)
	}

	args = cmdFlags.Args()
	numArgs := len(args)
	if numArgs < 1 {
		c.logger.Error("expected at least 1 args (device binary)", "args", args)
		return 1
	} else if numArgs > 2 {
		c.logger.Error("expected at most 2 args (device binary and config file)", "args", args)
		return 1
	}

	binary := args[0]
	var config []byte
	if numArgs == 2 {
		var err error
		config, err = ioutil.ReadFile(args[1])
		if err != nil {
			c.logger.Error("failed to read config file", "error", err)
			return 1
		}

		c.logger.Trace("read config", "config", string(config))
	}

	// Get the plugin
	dev, cleanup, err := c.getDevicePlugin(binary)
	if err != nil {
		c.logger.Error("failed to launch device plugin", "error", err)
		return 1
	}
	defer cleanup()
	c.dev = dev

	spec, err := c.getSpec()
	if err != nil {
		c.logger.Error("failed to get config spec", "error", err)
		return 1
	}
	c.spec = spec

	if err := c.setConfig(spec, config); err != nil {
		c.logger.Error("failed to set config", "error", err)
		return 1
	}

	if err := c.startRepl(); err != nil {
		c.logger.Error("error interacting with plugin", "error", err)
		return 1
	}

	return 0
}

func (c *Device) getDevicePlugin(binary string) (device.DevicePlugin, func(), error) {
	// Launch the plugin
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			base.PluginTypeBase:   &base.PluginBase{},
			base.PluginTypeDevice: &device.PluginDevice{},
		},
		Cmd:              exec.Command(binary),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           c.logger,
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, err
	}

	// Request the plugin
	raw, err := rpcClient.Dispense(base.PluginTypeDevice)
	if err != nil {
		client.Kill()
		return nil, nil, err
	}

	// We should have a KV store now! This feels like a normal interface
	// implementation but is in fact over an RPC connection.
	dev := raw.(device.DevicePlugin)
	return dev, func() { client.Kill() }, nil
}

func (c *Device) getSpec() (hcldec.Spec, error) {
	// Get the schema so we can parse the config
	spec, err := c.dev.ConfigSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to get config schema: %v", err)
	}

	c.logger.Trace("device spec", "spec", hclog.Fmt("% #v", pretty.Formatter(spec)))

	// Convert the schema
	schema, diag := hclspec.Convert(spec)
	if diag.HasErrors() {
		errStr := "failed to convert HCL schema: "
		for _, err := range diag.Errs() {
			errStr = fmt.Sprintf("%s\n* %s", errStr, err.Error())
		}
		return nil, errors.New(errStr)
	}

	return schema, nil
}

func (c *Device) setConfig(spec hcldec.Spec, config []byte) error {
	// Parse the config into hcl
	configVal, err := hclConfigToInterface(config)
	if err != nil {
		return err
	}

	c.logger.Trace("raw hcl config", "config", hclog.Fmt("% #v", pretty.Formatter(configVal)))

	ctx := &hcl2.EvalContext{
		Functions: shared.GetStdlibFuncs(),
	}

	val, diag := shared.ParseHclInterface(configVal, spec, ctx)
	if diag.HasErrors() {
		errStr := "failed to parse config"
		for _, err := range diag.Errs() {
			errStr = fmt.Sprintf("%s\n* %s", errStr, err.Error())
		}
		return errors.New(errStr)
	}
	c.logger.Trace("parsed hcl config", "config", hclog.Fmt("% #v", pretty.Formatter(val)))

	cdata, err := msgpack.Marshal(val, val.Type())
	if err != nil {
		return err
	}

	c.logger.Trace("msgpack config", "config", string(cdata))
	if err := c.dev.SetConfig(cdata); err != nil {
		return err
	}

	return nil
}

func hclConfigToInterface(config []byte) (interface{}, error) {
	if len(config) == 0 {
		return map[string]interface{}{}, nil
	}

	// Parse as we do in the jobspec parser
	root, err := hcl.Parse(string(config))
	if err != nil {
		return nil, fmt.Errorf("failed to hcl parse the config: %v", err)
	}

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("root should be an object")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, list.Items[0]); err != nil {
		return nil, fmt.Errorf("failed to decode object: %v", err)
	}

	return m["config"], nil
}

func (c *Device) startRepl() error {
	// Start the output goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fingerprint := make(chan context.Context)
	stats := make(chan context.Context)
	reserve := make(chan []string)
	go c.replOutput(ctx, fingerprint, stats, reserve)

	c.Ui.Output("> Availabile commands are: exit(), fingerprint(), stop_fingerprint(), stats(), stop_stats(), reserve(id1, id2, ...)")
	var fingerprintCtx, statsCtx context.Context
	var fingerprintCancel, statsCancel context.CancelFunc
	for {
		in, err := c.Ui.Ask("> ")
		if err != nil {
			return err
		}

		switch {
		case in == "exit()":
			return nil
		case in == "fingerprint()":
			if fingerprintCtx != nil {
				continue
			}
			fingerprintCtx, fingerprintCancel = context.WithCancel(ctx)
			fingerprint <- fingerprintCtx
		case in == "stop_fingerprint()":
			if fingerprintCtx == nil {
				continue
			}
			fingerprintCancel()
			fingerprintCtx = nil
		case in == "stats()":
			if statsCtx != nil {
				continue
			}
			statsCtx, statsCancel = context.WithCancel(ctx)
			stats <- statsCtx
		case in == "stop_stats()":
			if statsCtx == nil {
				continue
			}
			statsCancel()
			statsCtx = nil
		case strings.HasPrefix(in, "reserve(") && strings.HasSuffix(in, ")"):
			listString := strings.TrimSuffix(strings.TrimPrefix(in, "reserve("), ")")
			ids := strings.Split(strings.TrimSpace(listString), ",")
			reserve <- ids
		default:
			c.Ui.Error(fmt.Sprintf("> Unknown command %q", in))
		}
	}

	return nil
}

func (c *Device) replOutput(ctx context.Context, startFingerprint, startStats <-chan context.Context, reserve <-chan []string) {
	var fingerprint <-chan *device.FingerprintResponse
	var stats <-chan *device.StatsResponse
	for {
		select {
		case <-ctx.Done():
			return
		case ctx := <-startFingerprint:
			var err error
			fingerprint, err = c.dev.Fingerprint(ctx)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("fingerprint: %s", err))
				os.Exit(1)
			}
		case resp, ok := <-fingerprint:
			if !ok {
				c.Ui.Output("> fingerprint: fingerprint output closed")
				fingerprint = nil
				continue
			}

			if resp == nil {
				c.Ui.Warn("> fingerprint: received nil result")
				os.Exit(1)
			}

			c.Ui.Output(fmt.Sprintf("> fingerprint: % #v", pretty.Formatter(resp)))
		case ctx := <-startStats:
			var err error
			stats, err = c.dev.Stats(ctx)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("stats: %s", err))
				os.Exit(1)
			}
		case resp, ok := <-stats:
			if !ok {
				c.Ui.Output("> stats: stats output closed")
				stats = nil
				continue
			}

			if resp == nil {
				c.Ui.Warn("> stats: received nil result")
				os.Exit(1)
			}

			c.Ui.Output(fmt.Sprintf("> stats: % #v", pretty.Formatter(resp)))
		case ids := <-reserve:
			resp, err := c.dev.Reserve(ids)
			if err != nil {
				c.Ui.Warn(fmt.Sprintf("> reserve(%s): %v", strings.Join(ids, ", "), err))
			} else {
				c.Ui.Output(fmt.Sprintf("> reserve(%s): % #v", strings.Join(ids, ", "), pretty.Formatter(resp)))
			}
		}
	}
}
