package jobspec2

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	"github.com/zclconf/go-cty/cty/gocty"
)

func TestParse(t *testing.T) {
	jobStr := `
job "example" {
  datacenters = ["dc1"]

  group "cache" {
    task "redis" {
      driver = "hello ${  docker.asdf.qwer } asdf"

config {
hello = "a"
gc {
gc_key = "asdf"
container {
wht = "m"
}
}
}
      resources {
        cpu    = 500
        memory = 256

        network {
          mbits = 10
          port "db" {}
        }
      }
    }
  }
}
`

	job, err := Parse(strings.NewReader(jobStr))
	require.NoError(t, err)
	pretty.Println(job)
}

func TestBasic(t *testing.T) {
	f, err := os.Open("./test-fixtures/basic.hcl")
	require.NoError(t, err)
	defer f.Close()

	job, err := Parse(f)
	require.NoError(t, err)
	fmt.Println("### ", job.ID, job.Name)
	//fmt.Println(job)
	//pretty.Println(job)
	_ = job
}

func TestEquavalency(t *testing.T) {
	fis, err := ioutil.ReadDir("./test-fixtures")
	require.NoError(t, err)

	for _, fi := range fis {
		name := fi.Name()
		if strings.Contains(name, "bad") ||
			strings.HasPrefix(name, ".") {
			continue
		}

		t.Run(name, func(t *testing.T) {
			f, err := os.Open("./test-fixtures/" + name)
			require.NoError(t, err)
			defer f.Close()

			job1, err := jobspec.Parse(f)
			if err != nil {
				t.Skip("file is not parsable in v1")
			}

			f.Seek(0, 0)

			job2, err := Parse(f)
			require.NoError(t, err)

			require.Equal(t, job1, job2)
		})
	}
}

func TestMine(t *testing.T) {
	var v time.Duration

	c, _ := gocty.ImpliedType(&v)
	fmt.Printf("%#+v %v\n", c, reflect.TypeOf(v))
}
func TestHCL2_Spec(t *testing.T) {
	config := `
job "example" {
group "test" {
volume "foo" { source = "hello" }
}
}
`

	spec := hcldec.ObjectSpec(map[string]hcldec.Spec{
		"job": &hcldec.BlockSpec{
			TypeName: "job",
			//LabelNames: []string{},
			Nested: &hcldec.TupleSpec{
				&hcldec.BlockLabelSpec{Index: 0, Name: "job_name"},
				&hcldec.BlockAttrsSpec{TypeName: "meta", ElementType: cty.DynamicPseudoType},
			},
		},
	})

	file, diags := hclparse.NewParser().ParseHCL([]byte(config), "test.hcl")
	require.Empty(t, diags)

	ctx := &hcl.EvalContext{}
	v, diags := hcldec.Decode(file.Body, spec, ctx)
	fmt.Println("### V\n", v.GetAttr("job").Type().GoString())
	fmt.Println("### DIAGs\n", diags)

}

func TestHCL2_GoHCL(t *testing.T) {
	config := `job "example" {
name = "qwer"
group "asdf" {}
vault {
namespace = "whatever"
}
task "hello" {}
}`
	type Job struct {
		Label string `hcl:",label"`
	}

	type Target struct {
		Job JobWrapper `hcl:"job,block"`
	}

	file, diags := hclparse.NewParser().ParseHCL([]byte(config), "test.hcl")
	require.Empty(t, diags)
	//fmt.Println("#### block ", pretty.Sprint(file.Body))
	//return

	ctx := &hcl.EvalContext{
		Functions: map[string]function.Function{
			"upper": stdlib.UpperFunc,
		},
	}
	var v Target
	err := gohcl.DecodeBody(file.Body, ctx, &v)
	fmt.Println("### V\n", pretty.Sprint(v))
	fmt.Println("### DIAGs\n", err)

}

func TestHCLSchema(t *testing.T) {
	s, _ := gohcl.ImpliedBodySchema(JobWrapper{})
	pretty.Println(s)
}
