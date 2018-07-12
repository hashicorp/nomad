package command

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	getter "github.com/hashicorp/go-getter"
	"github.com/mitchellh/cli"
)

func init() {
	getter.Getters["file"].(*getter.FileGetter).Copy = true
}

func ProvisionCommandFactory() (cli.Command, error) {
	return &Provision{}, nil
}

type Provision struct {
}

func (c *Provision) Help() string {
	helpText := `
Usage: nomad-e2e provision <provider> <environment>

  Uses terraform to provision a target test environment to use
  for end-to-end testing.

  The output is a list of environment variables used to configure
  various api clients such as Nomad, Consul and Vault.

Provision Options:

  -env-path
    Sets the path for where to search for test environment configuration.
	This defaults to './environments/'.

  -nomad-binary
    Sets the target nomad-binary to use when provisioning a nomad cluster.
	The binary is retrieved by go-getter and can therefore be a local file
	path, remote http url, or other support go-getter uri.

  -nomad-checksum
    If set, will ensure the binary from -nomad-binary matches the given
	checksum.

  -destroy
    If set, will destroy the target environment.

  -tf-path
    Sets the path for which terraform state files are stored. Defaults to
	the current working directory.
`
	return strings.TrimSpace(helpText)
}

func (c *Provision) Synopsis() string {
	return "Provisions the target testing environment"
}

func (c *Provision) Run(args []string) int {
	var envPath string
	var nomadBinary string
	var destroy bool
	var tfPath string
	cmdFlags := flag.NewFlagSet("provision", flag.ContinueOnError)
	cmdFlags.Usage = func() { log.Println(c.Help()) }
	cmdFlags.StringVar(&envPath, "env-path", "./environments/", "Path to e2e environment terraform configs")
	cmdFlags.StringVar(&nomadBinary, "nomad-binary", "", "")
	cmdFlags.BoolVar(&destroy, "destroy", false, "")
	cmdFlags.StringVar(&tfPath, "tf-path", "", "")

	if err := cmdFlags.Parse(args); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	args = cmdFlags.Args()
	if len(args) != 2 {
		log.Fatalf("expected 2 args, but got: %v", args)
		log.Fatal(c.Help())
	}

	env, err := newEnv(envPath, args[0], args[1], tfPath)
	if err != nil {
		log.Fatal(err)
	}

	if destroy {
		if err := env.destroy(); err != nil {
			log.Fatal(err)
			return 1
		}
		fmt.Println("Environment successfully destroyed")
		return 0
	}

	// Use go-getter to fetch the nomad binary
	nomadPath, err := c.fetchBinary(nomadBinary)
	defer os.RemoveAll(nomadPath)

	results, err := env.provision(nomadPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(strings.TrimSpace(`
NOMAD_ADDR=%s
	`), results.nomadAddr)

	return 0
}

// Fetches the nomad binary and returns the temporary directory where it exists
func (c *Provision) fetchBinary(bin string) (string, error) {
	nomadBinaryDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}

	if err = getter.GetFile(path.Join(nomadBinaryDir, "nomad"), bin); err != nil {
		return "", fmt.Errorf("failed to get nomad binary: %v", err)
	}

	return nomadBinaryDir, nil
}

type environment struct {
	path     string
	provider string
	name     string

	tf      string
	tfPath  string
	tfState string
}

type envResults struct {
	nomadAddr string
}

func newEnv(envPath, provider, name, tfStatePath string) (*environment, error) {
	// Make sure terraform is on the PATH
	tf, err := exec.LookPath("terraform")
	if err != nil {
		return nil, fmt.Errorf("failed to lookup terraform binary: %v", err)
	}

	// set the path to the terraform module
	tfPath := path.Join(envPath, provider, name)
	log.Printf("[DEBUG] provision: using tf path %s", tfPath)
	if _, err := os.Stat(tfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to lookup terraform configuration dir %s: %v", tfPath, err)
	}

	// set the path to state file
	tfState := path.Join(tfStatePath, fmt.Sprintf("e2e.%s.%s.tfstate", provider, name))

	env := &environment{
		path:     envPath,
		provider: provider,
		name:     name,
		tf:       tf,
		tfPath:   tfPath,
		tfState:  tfState,
	}
	return env, nil
}

// provision calls terraform to setup the environment with the given nomad binary
func (env *environment) provision(nomadPath string) (*envResults, error) {
	tfArgs := []string{"apply", "-auto-approve", "-input=false", "-no-color",
		"-state", env.tfState,
		"-var", fmt.Sprintf("nomad_binary=%s", path.Join(nomadPath, "nomad")),
		env.tfPath,
	}

	// Setup the 'terraform apply' command
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, env.tf, tfArgs...)

	// Funnel the stdout/stderr to logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	// Run 'terraform apply'
	cmd.Start()
	go tfLog("tf.stderr", stderr)
	go tfLog("tf.stdout", stdout)

	err = cmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("terraform exited with a non-zero status: %v", err)
	}

	// Setup and run 'terraform output' to get the module output
	cmd = exec.CommandContext(ctx, env.tf, "output", "-json", "-state", env.tfState)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform exited with a non-zero status: %v", err)
	}

	// Parse the json and pull out results
	tfOutput := make(map[string]map[string]interface{})
	err = json.Unmarshal(out, &tfOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse terraform output: %v", err)
	}

	results := &envResults{}
	if nomadAddr, ok := tfOutput["nomad_addr"]; ok {
		results.nomadAddr = nomadAddr["value"].(string)
	}

	return results, nil
}

//destroy calls terraform to destroy the environment
func (env *environment) destroy() error {
	tfArgs := []string{"destroy", "-auto-approve", "-no-color",
		"-state", env.tfState,
		"-var", "nomad_binary=",
		env.tfPath,
	}
	cmd := exec.Command(env.tf, tfArgs...)

	// Funnel the stdout/stderr to logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	// Run 'terraform destroy'
	cmd.Start()
	go tfLog("tf.stderr", stderr)
	go tfLog("tf.stdout", stdout)

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("terraform exited with a non-zero status: %v", err)
	}

	return nil
}

func tfLog(prefix string, r io.ReadCloser) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log.Printf("[DEBUG] provision.%s: %s", prefix, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[WARN] provision.%s: %v", err)
	}

}
