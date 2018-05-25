// +build !windows

package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"plugin"
	"strings"
)

const (
	customDriversDir = "drivers"
	customDriverConstructorName = "NewDriver"
)

func init() {
	files, err := findCustomDrivers(customDriversDir)
	if err != nil {
		fmt.Println(err)
	}

	err = loadCustomDrivers(files, goPluginNewDriver)
	if err != nil {
		fmt.Println(err)
	}
}

func goPluginNewDriver(file string) (interface{}, error) {
	plug, err := plugin.Open(file)
	if err != nil {
		return nil, err
	}

	constructorLookup, err := plug.Lookup(customDriverConstructorName)
	if err != nil {
		return constructorLookup, err
	}

	return nil, nil
}

func findCustomDrivers(dir string) (files []string, err error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return files, nil
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return files, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".so") {
			pluginName := strings.TrimSuffix(file.Name(), ".so")
			files = append(files, path.Join(dir, pluginName))
		}
	}

	return files, nil
	
}

type pluginFactory func(string) (interface{}, error)

func loadCustomDrivers(files []string, plugin pluginFactory) error {
	for _, file := range files {
			plug, err := plugin(file)
			if err != nil {
				fmt.Println(err)
				return err
			}

			var factory Factory
			factory, ok := plug.(Factory)
			if !ok {
				fmt.Println("unexpected type from module symbol ", factory)
				return err
			}

			BuiltinDrivers[pluginName] = factory
		}
	}

	return nil
}
