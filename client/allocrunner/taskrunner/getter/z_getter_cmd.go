package getter

import (
	"os"

	"github.com/hashicorp/nomad/helper/subproc"
)

const (
	// SubCommand is the first argument to the clone of the nomad
	// agent process for downloading artifacts.
	SubCommand = "artifact-isolation"
)

func init() {
	subproc.Do(SubCommand, func() int {

		// get client and artifact configuration from standard IO
		env := new(parameters)
		if err := env.read(os.Stdin); err != nil {
			subproc.Print("failed to read configuration: %v", err)
			return subproc.ExitFailure
		}

		// create context with the overall timeout
		ctx, cancel := subproc.Context(env.deadline())
		defer cancel()

		// force quit after maximum timeout exceeded
		subproc.SetExpiration(ctx)

		// sandbox the host filesystem for this process
		dir := env.TaskDir
		if err := lockdown(dir); err != nil {
			subproc.Print("failed to sandbox getter process: %v", err)
			return subproc.ExitFailure
		}

		// create the go-getter client
		// options were already transformed into url query parameters
		// headers were already replaced and are usable now
		c := env.client(ctx)

		// run the go-getter client
		if err := c.Get(); err != nil {
			subproc.Print("failed to download artifact: %v", err)
			return subproc.ExitFailure
		}

		subproc.Print("artifact download was a success")
		return subproc.ExitSuccess
	})
}
