package main

import (
	"net/http"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (v *v1api) getNamespacePaths() []*Path {
	tags := []string{"Namespaces"}

	// TODO: add to nomad-enterprise repo
	return []*Path{
		//s.mux.HandleFunc("/v1/namespaces", s.wrap(s.NamespacesRequest))
		{
			Template: "/namespaces",
			Operations: []*Operation{
				newOperation(http.MethodGet, "NamespaceRequest", tags, "GetNamespaces",
					newRequestBody(objectSchema, structs.NamespaceListRequest{}),
					queryOptions,
					newResponseConfig(200, arraySchema, api.Namespace{}, queryMeta,
						"GetNamespacesResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/namespace/", s.wrap(s.NamespaceSpecificRequest))
		{
			Template: "/namespace/{namespaceName}",
			Operations: []*Operation{
				newOperation(http.MethodGet, "namespaceQuery", tags, "GetNamespace",
					nil,
					append(queryOptions, &NamespaceNameParam),
					newResponseConfig(200, objectSchema, api.Namespace{}, queryMeta,
						"GetNamespaceResponse"),
				),
				newOperation(http.MethodPost, "namespaceUpdate", tags, "PostNamespace",
					newRequestBody(objectSchema, api.Namespace{}),
					append(writeOptions, &NamespaceNameParam),
					newResponseConfig(200, "", nil, writeMeta,
						"PostNamespaceResponse"),
				),
				newOperation(http.MethodDelete, "namespaceDelete", tags, "DeleteNamespace",
					nil,
					append(writeOptions, &NamespaceNameParam),
					newResponseConfig(200, "", nil, writeMeta,
						"DeleteNamespaceResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/namespace", s.wrap(s.NamespaceCreateRequest)) routes to namespaceUpdate
		{
			Template: "/namespace",
			Operations: []*Operation{
				newOperation(http.MethodPost, "namespaceUpdate", tags, "CreateNamespace",
					nil,
					writeOptions,
					newResponseConfig(200, "", nil, writeMeta,
						"CreateNamespaceResponse"),
				),
			},
		},
	}
}
