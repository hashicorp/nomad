package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"plugin"
	"strings"
)

const customDriversDir = "drivers"

func init() {
	if _, err := os.Stat(customDriversDir); os.IsNotExist(err) {
		return
	}

	dirs, err := ioutil.ReadDir()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, dir := range dirs {
		if strings.HasSuffix(dir.FileName, ".so") {
			pluginName := strings.StripSuffix(dir.FileName, ".so")

			plug, err := plugin.Open(mod)
			if err != nil {
				fmt.Println(err)
				continue
			}

			constructorLookup, err := plug.Lookup("NewDriver")
			if err != nil {
				fmt.Println(err)
				continue
			}

			var factory Factory
			factory, ok := constructorLookup.(Factory)
			if !ok {
				fmt.Println("unexpected type from module symbol ", factory)
				continue
			}

			BuiltinDrivers[pluginName] = factory
		}
	}
}
