package jobspec

import (
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/mapstructure"
)

func ParseNetwork(o *ast.ObjectList) (*api.NetworkResource, error) {
	if len(o.Items) > 1 {
		return nil, fmt.Errorf("only one 'network' resource allowed")
	}

	// Check for invalid keys
	valid := []string{
		"mode",
		"mbits",
		"port",
	}
	if err := helper.CheckHCLKeys(o.Items[0].Val, valid); err != nil {
		return nil, multierror.Prefix(err, "network ->")
	}

	var r api.NetworkResource
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Items[0].Val); err != nil {
		return nil, err
	}
	if err := mapstructure.WeakDecode(m, &r); err != nil {
		return nil, err
	}

	var networkObj *ast.ObjectList
	if ot, ok := o.Items[0].Val.(*ast.ObjectType); ok {
		networkObj = ot.List
	} else {
		return nil, fmt.Errorf("should be an object")
	}
	if err := parsePorts(networkObj, &r); err != nil {
		return nil, multierror.Prefix(err, "network, ports ->")
	}

	return &r, nil
}

func parsePorts(networkObj *ast.ObjectList, nw *api.NetworkResource) error {
	// Check for invalid keys
	valid := []string{
		"mbits",
		"port",
		"mode",
	}
	if err := helper.CheckHCLKeys(networkObj, valid); err != nil {
		return err
	}

	portsObjList := networkObj.Filter("port")
	knownPortLabels := make(map[string]bool)
	for _, port := range portsObjList.Items {
		if len(port.Keys) == 0 {
			return fmt.Errorf("ports must be named")
		}
		label := port.Keys[0].Token.Value().(string)
		if !reDynamicPorts.MatchString(label) {
			return errPortLabel
		}
		l := strings.ToLower(label)
		if knownPortLabels[l] {
			return fmt.Errorf("found a port label collision: %s", label)
		}
		var p map[string]interface{}
		var res api.Port
		if err := hcl.DecodeObject(&p, port.Val); err != nil {
			return err
		}
		if err := mapstructure.WeakDecode(p, &res); err != nil {
			return err
		}
		res.Label = label
		if res.Value > 0 {
			nw.ReservedPorts = append(nw.ReservedPorts, res)
		} else {
			nw.DynamicPorts = append(nw.DynamicPorts, res)
		}
		knownPortLabels[l] = true
	}
	return nil
}
