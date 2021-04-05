package consul

import (
	"sort"
	"strings"
)

// NamespacesClient is a wrapper for the Consul NamespacesAPI, that is used to
// deal with Consul OSS vs Consul Enterprise behavior in listing namespaces.
type NamespacesClient struct {
	namespacesAPI NamespaceAPI
}

// NewNamespacesClient returns a NamespacesClient backed by a NamespaceAPI.
func NewNamespacesClient(namespacesAPI NamespaceAPI) *NamespacesClient {
	return &NamespacesClient{
		namespacesAPI: namespacesAPI,
	}
}

// List returns a list of Consul Namespaces.
//
// If using Consul OSS, the list is a single element with the "default" namespace,
// even though the response from Consul OSS is an error.
func (ns *NamespacesClient) List() ([]string, error) {
	namespaces, _, err := ns.namespacesAPI.List(nil)
	if err != nil {
		// check if the error was a 404, indicating Consul is the OSS version
		// which does not have the /v1/namespace handler
		if strings.Contains(err.Error(), "response code: 404") {
			return []string{"default"}, nil
		}
		return nil, err
	}

	result := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		result = append(result, namespace.Name)
	}
	sort.Strings(result)
	return result, nil
}
