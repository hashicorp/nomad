package jobspec2

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/stretchr/testify/require"
)

func TestEquivalentToHCL1(t *testing.T) {
	hclSpecDir := "../jobspec/test-fixtures/"
	fis, err := ioutil.ReadDir(hclSpecDir)
	require.NoError(t, err)

	for _, fi := range fis {
		name := fi.Name()

		t.Run(name, func(t *testing.T) {
			f, err := os.Open(hclSpecDir + name)
			require.NoError(t, err)
			defer f.Close()

			job1, err := jobspec.Parse(f)
			if err != nil {
				t.Skip("file is not parsable in v1")
			}

			f.Seek(0, 0)

			job2, err := Parse(name, f)
			require.NoError(t, err)

			require.Equal(t, job1, job2)
		})
	}
}

func TestEquivalentToHCL1_ComplexConfig(t *testing.T) {
	name := "./test-fixtures/config-compatibility.hcl"
	f, err := os.Open(name)
	require.NoError(t, err)
	defer f.Close()

	job1, err := jobspec.Parse(f)
	require.NoError(t, err)

	f.Seek(0, 0)

	job2, err := Parse(name, f)
	require.NoError(t, err)

	require.Equal(t, job1, job2)
}

func TestParse_VarsAndFunctions(t *testing.T) {
	hcl := `
variables {
  region_var = "default"
}
job "example" {
  datacenters = [for s in ["dc1", "dc2"] : upper(s)]
  region      = var.region_var
}
`

	out, err := ParseWithConfig(&ParseConfig{
		Path:    "input.hcl",
		Body:    []byte(hcl),
		ArgVars: []string{"region_var=aug"},
		AllowFS: true,
	})
	require.NoError(t, err)

	require.Equal(t, []string{"DC1", "DC2"}, out.Datacenters)
	require.NotNil(t, out.Region)
	require.Equal(t, "aug", *out.Region)
}

func TestParse_VariablesDefaultsAndSet(t *testing.T) {
	hcl := `
variables {
  region_var = "default_region"
}

variable "dc_var" {
  default = "default_dc"
}

job "example" {
  datacenters = [var.dc_var]
  region      = var.region_var
}
`

	t.Run("defaults", func(t *testing.T) {
		out, err := ParseWithConfig(&ParseConfig{
			Path:    "input.hcl",
			Body:    []byte(hcl),
			AllowFS: true,
		})
		require.NoError(t, err)

		require.Equal(t, []string{"default_dc"}, out.Datacenters)
		require.NotNil(t, out.Region)
		require.Equal(t, "default_region", *out.Region)
	})

	t.Run("set via -var args", func(t *testing.T) {
		out, err := ParseWithConfig(&ParseConfig{
			Path:    "input.hcl",
			Body:    []byte(hcl),
			ArgVars: []string{"dc_var=set_dc", "region_var=set_region"},
			AllowFS: true,
		})
		require.NoError(t, err)

		require.Equal(t, []string{"set_dc"}, out.Datacenters)
		require.NotNil(t, out.Region)
		require.Equal(t, "set_region", *out.Region)
	})

	t.Run("set via envvars", func(t *testing.T) {
		out, err := ParseWithConfig(&ParseConfig{
			Path: "input.hcl",
			Body: []byte(hcl),
			Envs: []string{
				"NOMAD_VAR_dc_var=set_dc",
				"NOMAD_VAR_region_var=set_region",
			},
			AllowFS: true,
		})
		require.NoError(t, err)

		require.Equal(t, []string{"set_dc"}, out.Datacenters)
		require.NotNil(t, out.Region)
		require.Equal(t, "set_region", *out.Region)
	})

	t.Run("set via var-files", func(t *testing.T) {
		varFile, err := ioutil.TempFile("", "")
		require.NoError(t, err)
		defer os.Remove(varFile.Name())

		content := `dc_var = "set_dc"
region_var = "set_region"`
		_, err = varFile.WriteString(content)
		require.NoError(t, err)

		out, err := ParseWithConfig(&ParseConfig{
			Path:     "input.hcl",
			Body:     []byte(hcl),
			VarFiles: []string{varFile.Name()},
			AllowFS:  true,
		})
		require.NoError(t, err)

		require.Equal(t, []string{"set_dc"}, out.Datacenters)
		require.NotNil(t, out.Region)
		require.Equal(t, "set_region", *out.Region)
	})
}

// TestParse_UnknownVariables asserts that unknown variables are left intact for further processing
func TestParse_UnknownVariables(t *testing.T) {
	hcl := `
variables {
  region_var = "default"
}
job "example" {
  datacenters = [for s in ["dc1", "dc2"] : upper(s)]
  region      = var.region_var
  meta {
    known_var   = "${var.region_var}"
    unknown_var = "${UNKNOWN}"
  }
}
`

	out, err := ParseWithConfig(&ParseConfig{
		Path:    "input.hcl",
		Body:    []byte(hcl),
		ArgVars: []string{"region_var=aug"},
		AllowFS: true,
	})
	require.NoError(t, err)

	meta := map[string]string{
		"known_var":   "aug",
		"unknown_var": "${UNKNOWN}",
	}

	require.Equal(t, meta, out.Meta)
}

// TestParse_UnsetVariables asserts that variables that have neither types nor
// values return early instead of panicking.
func TestParse_UnsetVariables(t *testing.T) {
	hcl := `
variable "region_var" {}
job "example" {
  datacenters = [for s in ["dc1", "dc2"] : upper(s)]
  region      = var.region_var
}
`

	_, err := ParseWithConfig(&ParseConfig{
		Path:    "input.hcl",
		Body:    []byte(hcl),
		ArgVars: []string{},
		AllowFS: true,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "Unset variable")
}

func TestParse_Locals(t *testing.T) {
	hcl := `
variables {
  region_var = "default_region"
}

locals {
  # literal local
  dc = "local_dc"
  # local that depends on a variable
  region = "${var.region_var}.example"
}

job "example" {
  datacenters = [local.dc]
  region      = local.region
}
`

	t.Run("defaults", func(t *testing.T) {
		out, err := ParseWithConfig(&ParseConfig{
			Path:    "input.hcl",
			Body:    []byte(hcl),
			AllowFS: true,
		})
		require.NoError(t, err)

		require.Equal(t, []string{"local_dc"}, out.Datacenters)
		require.NotNil(t, out.Region)
		require.Equal(t, "default_region.example", *out.Region)
	})

	t.Run("set via -var argments", func(t *testing.T) {
		out, err := ParseWithConfig(&ParseConfig{
			Path:    "input.hcl",
			Body:    []byte(hcl),
			ArgVars: []string{"region_var=set_region"},
			AllowFS: true,
		})
		require.NoError(t, err)

		require.Equal(t, []string{"local_dc"}, out.Datacenters)
		require.NotNil(t, out.Region)
		require.Equal(t, "set_region.example", *out.Region)
	})
}

func TestParse_FileOperators(t *testing.T) {
	hcl := `
job "example" {
  region      = file("parse_test.go")
}
`

	t.Run("enabled", func(t *testing.T) {
		out, err := ParseWithConfig(&ParseConfig{
			Path:    "input.hcl",
			Body:    []byte(hcl),
			ArgVars: nil,
			AllowFS: true,
		})
		require.NoError(t, err)

		expected, err := ioutil.ReadFile("parse_test.go")
		require.NoError(t, err)

		require.NotNil(t, out.Region)
		require.Equal(t, string(expected), *out.Region)
	})

	t.Run("disabled", func(t *testing.T) {
		_, err := ParseWithConfig(&ParseConfig{
			Path:    "input.hcl",
			Body:    []byte(hcl),
			ArgVars: nil,
			AllowFS: false,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "filesystem function disabled")
	})
}

func TestParseDynamic(t *testing.T) {
	hcl := `
job "example" {

  dynamic "group" {
    for_each = [
      { name = "groupA", idx = 1 },
      { name = "groupB", idx = 2 },
      { name = "groupC", idx = 3 },
    ]
    labels   = [group.value.name]

    content {
      count = group.value.idx

      service {
        port = group.value.name
      }

      task "simple" {
        driver  = "raw_exec"
        config {
          command = group.value.name
        }
        meta {
          VERSION = group.value.idx
        }
        env {
          ID = format("id:%s", group.value.idx)
        }
        resources {
          cpu = group.value.idx
        }
      }
    }
  }
}
`
	out, err := ParseWithConfig(&ParseConfig{
		Path:    "input.hcl",
		Body:    []byte(hcl),
		ArgVars: nil,
		AllowFS: false,
	})
	require.NoError(t, err)

	require.Len(t, out.TaskGroups, 3)
	require.Equal(t, "groupA", *out.TaskGroups[0].Name)
	require.Equal(t, "groupB", *out.TaskGroups[1].Name)
	require.Equal(t, "groupC", *out.TaskGroups[2].Name)
	require.Equal(t, 1, *out.TaskGroups[0].Tasks[0].Resources.CPU)
	require.Equal(t, "groupA", out.TaskGroups[0].Services[0].PortLabel)

	// interpolation inside maps
	require.Equal(t, "groupA", out.TaskGroups[0].Tasks[0].Config["command"])
	require.Equal(t, "1", out.TaskGroups[0].Tasks[0].Meta["VERSION"])
	require.Equal(t, "id:1", out.TaskGroups[0].Tasks[0].Env["ID"])
	require.Equal(t, "id:2", out.TaskGroups[1].Tasks[0].Env["ID"])
	require.Equal(t, "3", out.TaskGroups[2].Tasks[0].Meta["VERSION"])
}

func TestParse_InvalidScalingSyntax(t *testing.T) {
	cases := []struct {
		name        string
		expectedErr string
		hcl         string
	}{
		{
			"valid",
			"",
			`
job "example" {
  group "g1" {
    scaling {
      max  = 40
      type = "horizontal"
    }

    task "t1" {
      scaling "cpu" {
        max = 20
      }
      scaling "mem" {
        max = 15
      }
    }
  }
}
`,
		},
		{
			"group missing max",
			`argument "max" is required`,
			`
job "example" {
  group "g1" {
    scaling {
      #max  = 40
      type = "horizontal"
    }

    task "t1" {
      scaling "cpu" {
        max = 20
      }
      scaling "mem" {
        max = 15
      }
    }
  }
}
`,
		},
		{
			"group invalid type",
			`task group scaling policy had invalid type`,
			`
job "example" {
  group "g1" {
    scaling {
      max  = 40
      type = "invalid_type"
    }

    task "t1" {
      scaling "cpu" {
        max = 20
      }
      scaling "mem" {
        max = 15
      }
    }
  }
}
`,
		},
		{
			"task invalid label",
			`scaling policy name must be "cpu" or "mem"`,
			`
job "example" {
  group "g1" {
    scaling {
      max  = 40
      type = "horizontal"
    }

    task "t1" {
      scaling "not_cpu" {
        max = 20
      }
      scaling "mem" {
        max = 15
      }
    }
  }
}
`,
		},
		{
			"task duplicate blocks",
			`Duplicate scaling "cpu" block`,
			`
job "example" {
  group "g1" {
    scaling {
      max  = 40
      type = "horizontal"
    }

    task "t1" {
      scaling "cpu" {
        max = 20
      }
      scaling "cpu" {
        max = 15
      }
    }
  }
}
`,
		},
		{
			"task invalid type",
			`Invalid scaling policy type`,
			`
job "example" {
  group "g1" {
    scaling {
      max  = 40
      type = "horizontal"
    }

    task "t1" {
      scaling "cpu" {
        max  = 20
        type = "invalid"
      }
      scaling "mem" {
        max = 15
      }
    }
  }
}
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseWithConfig(&ParseConfig{
				Path:    c.name + ".hcl",
				Body:    []byte(c.hcl),
				AllowFS: false,
			})
			if c.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expectedErr)
			}
		})
	}
}

func TestParseJob_JobWithFunctionsAndLookups(t *testing.T) {
	hcl := `
variable "env" {
  description = "target environment for the job"
}

locals {
  environments = {
    prod    = { count = 20, dcs = ["prod-dc1", "prod-dc2"] },
    staging = { count = 3, dcs = ["dc1"] },
  }

  env = lookup(local.environments, var.env, { count = 0, dcs = [] })
}

job "job-webserver" {
  datacenters = local.env.dcs
  group "group-webserver" {
    count = local.env.count

    task "server" {
      driver = "docker"

      config {
        image = "hashicorp/http-echo"
        args  = ["-text", "Hello from ${var.env}"]
      }
    }
  }
}
`
	cases := []struct {
		env         string
		expectedJob *api.Job
	}{
		{
			"prod",
			&api.Job{
				ID:          stringToPtr("job-webserver"),
				Name:        stringToPtr("job-webserver"),
				Datacenters: []string{"prod-dc1", "prod-dc2"},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("group-webserver"),
						Count: intToPtr(20),

						Tasks: []*api.Task{
							{
								Name:   "server",
								Driver: "docker",

								Config: map[string]interface{}{
									"image": "hashicorp/http-echo",
									"args":  []interface{}{"-text", "Hello from prod"},
								},
							},
						},
					},
				},
			},
		},
		{
			"staging",
			&api.Job{
				ID:          stringToPtr("job-webserver"),
				Name:        stringToPtr("job-webserver"),
				Datacenters: []string{"dc1"},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("group-webserver"),
						Count: intToPtr(3),

						Tasks: []*api.Task{
							{
								Name:   "server",
								Driver: "docker",

								Config: map[string]interface{}{
									"image": "hashicorp/http-echo",
									"args":  []interface{}{"-text", "Hello from staging"},
								},
							},
						},
					},
				},
			},
		},
		{
			"unknown",
			&api.Job{
				ID:          stringToPtr("job-webserver"),
				Name:        stringToPtr("job-webserver"),
				Datacenters: []string{},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("group-webserver"),
						Count: intToPtr(0),

						Tasks: []*api.Task{
							{
								Name:   "server",
								Driver: "docker",

								Config: map[string]interface{}{
									"image": "hashicorp/http-echo",
									"args":  []interface{}{"-text", "Hello from unknown"},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			found, err := ParseWithConfig(&ParseConfig{
				Path:    "example.hcl",
				Body:    []byte(hcl),
				AllowFS: false,
				ArgVars: []string{"env=" + c.env},
			})
			require.NoError(t, err)
			require.Equal(t, c.expectedJob, found)
		})
	}
}

func TestParse_TaskEnvs(t *testing.T) {
	cases := []struct {
		name       string
		envSnippet string
		expected   map[string]string
	}{
		{
			"none",
			``,
			nil,
		},
		{
			"block",
			`
env {
  key = "value"
} `,
			map[string]string{"key": "value"},
		},
		{
			"attribute",
			`
env = {
  "key.dot"                = "val1"
  key_unquoted_without_dot = "val2"
} `,
			map[string]string{"key.dot": "val1", "key_unquoted_without_dot": "val2"},
		},
		{
			"attribute_colons",
			`env = {
  "key.dot" : "val1"
  key_unquoted_without_dot : "val2"
} `,
			map[string]string{"key.dot": "val1", "key_unquoted_without_dot": "val2"},
		},
		{
			"attribute_empty",
			`env = {}`,
			map[string]string{},
		},
		{
			"attribute_expression",
			`env = {for k in ["a", "b"]: k => "val-${k}" }`,
			map[string]string{"a": "val-a", "b": "val-b"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hcl := `
job "example" {
  group "group" {
    task "task" {
      driver = "docker"
      config {}

      ` + c.envSnippet + `
    }
  }
}`

			out, err := ParseWithConfig(&ParseConfig{
				Path: "input.hcl",
				Body: []byte(hcl),
			})
			require.NoError(t, err)

			require.Equal(t, c.expected, out.TaskGroups[0].Tasks[0].Env)
		})
	}
}

func TestParse_TaskEnvs_Multiple(t *testing.T) {
	hcl := `
job "example" {
  group "group" {
    task "task" {
      driver = "docker"
      config {}

      env = {"a": "b"}
      env {
        c = "d"
      }
    }
  }
}`

	_, err := ParseWithConfig(&ParseConfig{
		Path: "input.hcl",
		Body: []byte(hcl),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Duplicate env block")
}

func Test_TaskEnvs_Invalid(t *testing.T) {
	cases := []struct {
		name        string
		envSnippet  string
		expectedErr string
	}{
		{
			"attr: invalid expression",
			`env = { key = local.undefined_local }`,
			`does not have an attribute named "undefined_local"`,
		},
		{
			"block: invalid block expression",
			`env {
  for k in ["a", "b"]: k => k
}`,
			"Invalid block definition",
		},
		{
			"attr: not make sense",
			`env = [ "a" ]`,
			"Unsuitable value: map of string required",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hcl := `
job "example" {
  group "group" {
    task "task" {
      driver = "docker"
      config {}

      ` + c.envSnippet + `
    }
  }
}`
			_, err := ParseWithConfig(&ParseConfig{
				Path: "input.hcl",
				Body: []byte(hcl),
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), c.expectedErr)
		})
	}
}

func TestParse_Meta_Alternatives(t *testing.T) {
	hcl := ` job "example" {
  group "group" {
    task "task" {
      driver = "config"
      config {}

      meta {
        source = "task"
      }
    }

    meta {
      source = "group"

    }
  }

  meta {
    source = "job"
  }
}
`

	asBlock, err := ParseWithConfig(&ParseConfig{
		Path: "input.hcl",
		Body: []byte(hcl),
	})
	require.NoError(t, err)

	hclAsAttr := strings.ReplaceAll(hcl, "meta {", "meta = {")
	require.Equal(t, 3, strings.Count(hclAsAttr, "meta = {"))

	asAttr, err := ParseWithConfig(&ParseConfig{
		Path: "input.hcl",
		Body: []byte(hclAsAttr),
	})
	require.NoError(t, err)

	require.Equal(t, asBlock, asAttr)
	require.Equal(t, map[string]string{"source": "job"}, asBlock.Meta)
	require.Equal(t, map[string]string{"source": "group"}, asBlock.TaskGroups[0].Meta)
	require.Equal(t, map[string]string{"source": "task"}, asBlock.TaskGroups[0].Tasks[0].Meta)

}
