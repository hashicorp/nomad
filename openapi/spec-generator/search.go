package main

import (
	"github.com/hashicorp/nomad/api"
	"net/http"
)

func (v *v1api) getSearchPaths() []*Path {
	tags := []string{"Search"}

	return []*Path{
		//s.mux.HandleFunc("/v1/search/fuzzy", s.wrap(s.FuzzySearchRequest))
		{
			Template: "/search/fuzzy",
			Operations: []*Operation{
				newOperation(http.MethodPost, "newFuzzySearchRequest", tags, "GetFuzzySearch",
					newRequestBody(objectSchema, api.FuzzySearchRequest{}),
					queryOptions,
					newResponseConfig(200, objectSchema, api.FuzzySearchResponse{}, queryMeta, "GetFuzzySearchResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/search", s.wrap(s.SearchRequest))
		{
			Template: "/search",
			Operations: []*Operation{
				newOperation(http.MethodPost, "newSearchRequest", tags, "GetSearch",
					newRequestBody(objectSchema, api.SearchRequest{}),
					queryOptions,
					newResponseConfig(200, objectSchema, api.SearchResponse{}, queryMeta, "GetSearchResponse"),
				),
			},
		},
	}
}
