// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/posener/complete"
)

const (
	// DefaultHclVolumeInitName is the default name we use when initializing
	// the example volume file in HCL format
	DefaultHclVolumeInitName = "volume.hcl"

	// DefaultHclVolumeInitName is the default name we use when initializing
	// the example volume file in JSON format
	DefaultJsonVolumeInitName = "volume.json"
)

// VolumeInitCommand generates a new volume spec that you can customize to
// your liking, like vagrant init
type VolumeInitCommand struct {
	Meta
}

func (c *VolumeInitCommand) Help() string {
	helpText := `
Usage: nomad volume init <filename>

  Creates an example volume specification file that can be used as a starting
  point to customize further. If no filename is give, the default "volume.json"
  or "volume.hcl" will be used.

Init Options:

  -json
    Create an example JSON volume specification.
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeInitCommand) Synopsis() string {
	return "Create an example volume specification file"
}

func (c *VolumeInitCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-json": complete.PredictNothing,
	}
}

func (c *VolumeInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VolumeInitCommand) Name() string { return "volume init" }

func (c *VolumeInitCommand) Run(args []string) int {
	var jsonOutput bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&jsonOutput, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get no arguments
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes no arguments or one: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	fileName := DefaultHclVolumeInitName
	fileContent := defaultHclVolumeSpec
	if jsonOutput {
		fileName = DefaultJsonVolumeInitName
		fileContent = defaultJsonVolumeSpec
	}
	if len(args) == 1 {
		fileName = args[0]
	}

	// Check if the file already exists
	_, err := os.Stat(fileName)
	if err != nil && !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Failed to stat %q: %v", fileName, err))
		return 1
	}
	if !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Volume specification %q already exists", fileName))
		return 1
	}

	// Write out the example
	err = os.WriteFile(fileName, []byte(fileContent), 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write %q: %v", fileName, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example volume specification written to %s", fileName))
	return 0
}

var defaultHclVolumeSpec = strings.TrimSpace(`
id        = "ebs_prod_db1"
namespace = "default"
name      = "database"
type      = "csi"
plugin_id = "plugin_id"

# For 'nomad volume register', provide the external ID from the storage
# provider. This field should be omitted when creating a volume with
# 'nomad volume create'
external_id = "vol-23452345"

# For 'nomad volume create', specify a snapshot ID or volume to clone. You can
# specify only one of these two fields.
snapshot_id = "snap-12345"
# clone_id    = "vol-abcdef"

# Optional: for 'nomad volume create', specify a maximum and minimum capacity.
# Registering an existing volume will record but ignore these fields.
capacity_min = "10GiB"
capacity_max = "20G"

# Required (at least one): for 'nomad volume create', specify one or more
# capabilities to validate. Registering an existing volume will record but
# ignore these fields.
capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader"
  attachment_mode = "block-device"
}

# Optional: for 'nomad volume create', specify mount options to validate for
# 'attachment_mode = "file-system". Registering an existing volume will record
# but ignore these fields.
mount_options {
  fs_type     = "ext4"
  mount_flags = ["ro"]
}

# Optional: specify one or more locations where the volume must be accessible
# from. Refer to the plugin documentation for what segment values are supported.
topology_request {
  preferred {
    topology { segments { rack = "R1" } }
  }
  required {
    topology { segments { rack = "R1" } }
    topology { segments { rack = "R2", zone = "us-east-1a" } }
  }
}

# Optional: provide any secrets specified by the plugin.
secrets {
  example_secret = "xyzzy"
}

# Optional: provide a map of keys to string values expected by the plugin.
parameters {
  skuname = "Premium_LRS"
}

# Optional: for 'nomad volume register', provide a map of keys to string
# values expected by the plugin. This field will populated automatically by
# 'nomad volume create'.
context {
  endpoint = "http://192.168.1.101:9425"
}
`)

var defaultJsonVolumeSpec = strings.TrimSpace(`
{
  "id": "ebs_prod_db1",
  "namespace": "default",
  "name": "database",
  "type": "csi",
  "plugin_id": "plugin_id",
  "external_id": "vol-23452345",
  "snapshot_id": "snap-12345",
  "capacity_min": "10GiB",
  "capacity_max": "20G",
  "capability": [
    {
      "access_mode": "single-node-writer",
      "attachment_mode": "file-system"
    },
    {
      "access_mode": "single-node-reader",
      "attachment_mode": "block-device"
    }
  ],
  "context": [
    {
      "endpoint": "http://192.168.1.101:9425"
    }
  ],
  "mount_options": [
    {
      "fs_type": "ext4",
      "mount_flags": [
        "ro"
      ]
    }
  ],
  "topology_request": {
    "preferred": [
      {
        "topology": {
          "segments": {
            "rack": "R1"
          }
        }
      }
    ],
    "required": [
      {
        "topology": {
          "segments": {
            "rack": "R1"
          }
        }
      },
      {
        "topology": {
          "segments": {
            "rack": "R2",
            "zone": "us-east-1a"
          }
        }
      }
    ]
  },
  "parameters": [
    {
      "skuname": "Premium_LRS"
    }
  ],
  "secrets": [
    {
      "example_secret": "xyzzy"
    }
  ]
}
`)
