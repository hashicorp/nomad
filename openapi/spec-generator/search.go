package main

import (
	"github.com/hashicorp/nomad/api"
	"net/http"
)

func (v *v1api) getSearchPaths() []*apiPath {
	tags := []string{"Search"}

	return []*apiPath{
		//s.mux.HandleFunc("/v1/search/fuzzy", s.wrap(s.FuzzySearchRequest))
		{
			Template: "/search/fuzzy",
			Operations: []*operation{
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
			Operations: []*operation{
				newOperation(http.MethodPost, "newSearchRequest", tags, "GetSearch",
					newRequestBody(objectSchema, api.SearchRequest{}),
					queryOptions,
					newResponseConfig(200, objectSchema, api.SearchResponse{}, queryMeta, "GetSearchResponse"),
				),
			},
		},
	}
}
