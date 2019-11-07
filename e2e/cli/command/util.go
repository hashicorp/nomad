package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"time"

	getter "github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/helper/discover"
)

// fetchBinary fetches the nomad binary and returns the temporary directory where it exists
func fetchBinary(bin string) (string, error) {
	nomadBinaryDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}

	if bin == "" {
		bin, err = discover.NomadExecutable()
		if err != nil {
			return "", fmt.Errorf("failed to discover nomad binary: %v", err)
		}
	}

	dest := path.Join(nomadBinaryDir, "nomad")
	if runtime.GOOS == "windows" {
		dest = dest + ".exe"
	}

	if err = getter.GetFile(dest, bin); err != nil {
		return "", fmt.Errorf("failed to get nomad binary: %v", err)
	}

	return nomadBinaryDir, nil
}

func procWaitTimeout(p *os.Process, d time.Duration) error {
	stop := make(chan struct{})

	go func() {
		p.Wait()
		stop <- struct{}{}
	}()

	select {
	case <-stop:
		return nil
	case <-time.NewTimer(d).C:
		return fmt.Errorf("timeout waiting for process %d to exit", p.Pid)
	}
}
