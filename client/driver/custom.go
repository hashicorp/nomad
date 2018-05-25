// +build linux

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
	customDriversDir            = "drivers"
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

func goPluginNewDriver(file string) (Factory, error) {
	plug, err := plugin.Open(file)
	if err != nil {
		return nil, err
	}

	constructorLookup, err := plug.Lookup(customDriverConstructorName)
	if err != nil {
		return nil, err
	}

	var factory Factory
	factory, ok := constructorLookup.(Factory)
	if !ok {
		return nil, fmt.Errorf("unexpected type from module symbol %v - should be Factory", plug)
	}

	return factory, nil
}

func findCustomDrivers(dir string) (files []string, err error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return files, nil
	}

	dirFiles, err := ioutil.ReadDir(dir)
	if err != nil {
		return files, err
	}

	for _, file := range dirFiles {
		if strings.HasSuffix(file.Name(), ".so") {
			files = append(files, path.Join(dir, file.Name()))
		}
	}

	return files, nil

}

type pluginFactory func(string) (Factory, error)

func loadCustomDrivers(files []string, plugin pluginFactory) error {
	for _, file := range files {
		factory, err := plugin(file)
		if err != nil {
			return err
		}
		if err == nil {
			return fmt.Errorf("Nil plugin factory retured for %s", file)
		}

		pluginName := file
		slash := strings.LastIndex(pluginName, "/")
		if slash >= 0 && slash+1 < len(pluginName) {
			pluginName = pluginName[slash+1:]
		}
		pluginName = strings.TrimSuffix(pluginName, ".so")

		BuiltinDrivers[pluginName] = factory
	}
	return nil
}
