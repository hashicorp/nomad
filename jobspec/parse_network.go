package jobspec

import (
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/mapstructure"
)

// ParseNetwork parses a collection containing exactly one NetworkResource
func ParseNetwork(o *ast.ObjectList) (*api.NetworkResource, error) {
	if len(o.Items) > 1 {
		return nil, fmt.Errorf("only one 'network' resource allowed")
	}

	// Check for invalid keys
	valid := []string{
		"mode",
		"mbits",
		"dns",
		"port",
		"hostname",
	}
	if err := checkHCLKeys(o.Items[0].Val, valid); err != nil {
		return nil, multierror.Prefix(err, "network ->")
	}

	var r api.NetworkResource
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Items[0].Val); err != nil {
		return nil, err
	}

	delete(m, "dns")
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

	// Filter dns
	if dns := networkObj.Filter("dns"); len(dns.Items) > 0 {
		if len(dns.Items) > 1 {
			return nil, multierror.Prefix(fmt.Errorf("cannot have more than 1 dns block"), "network ->")
		}

		d, err := parseDNS(dns.Items[0])
		if err != nil {
			return nil, multierror.Prefix(err, "network ->")
		}

		r.DNS = d
	}

	return &r, nil
}

func parsePorts(networkObj *ast.ObjectList, nw *api.NetworkResource) error {
	portsObjList := networkObj.Filter("port")
	knownPortLabels := make(map[string]bool)
	for _, port := range portsObjList.Items {
		if len(port.Keys) == 0 {
			return fmt.Errorf("ports must be named")
		}

		// check for invalid keys
		valid := []string{
			"static",
			"to",
			"host_network",
		}
		if err := checkHCLKeys(port.Val, valid); err != nil {
			return err
		}

		label := port.Keys[0].Token.Value().(string)
		if !reDynamicPorts.MatchString(label) {
			return errPortLabel
		}
		l := strings.ToLower(label)
		if knownPortLabels[l] {
			return fmt.Errorf("found a port label collision: %s", label)
		}

		var res api.Port
		if err := hcl.DecodeObject(&res, port.Val); err != nil {
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

func parseDNS(dns *ast.ObjectItem) (*api.DNSConfig, error) {
	valid := []string{
		"servers",
		"searches",
		"options",
	}

	if err := checkHCLKeys(dns.Val, valid); err != nil {
		return nil, multierror.Prefix(err, "dns ->")
	}

	var dnsCfg api.DNSConfig
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, dns.Val); err != nil {
		return nil, err
	}

	if err := mapstructure.WeakDecode(m, &dnsCfg); err != nil {
		return nil, err
	}

	return &dnsCfg, nil
}
