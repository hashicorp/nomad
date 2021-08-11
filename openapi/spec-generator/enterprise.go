package main

import (
	"net/http"

	"github.com/hashicorp/nomad/api"
)

func (v *v1api) getEnterprisePaths() []*apiPath {
	tags := []string{"Enterprise"}

	// TODO: add to nomad-enterprise repo
	return []*apiPath{
		//s.mux.HandleFunc("/v1/sentinel/policies", s.wrap(s.entOnly))
		//s.mux.HandleFunc("/v1/sentinel/policy/", s.wrap(s.entOnly))
		//
		//s.mux.HandleFunc("/v1/quotas", s.wrap(s.entOnly))
		{
			Template: "/quotas",
			Operations: []*operation{
				newOperation(http.MethodGet, "QuotasRequests", tags, "GetQuotas",
					nil,
					queryOptions,
					newResponseConfig(200, arraySchema, api.Quotas{}, queryMeta,
						"GetQuotasResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/quota-usages", s.wrap(s.entOnly))
		//s.mux.HandleFunc("/v1/quota/", s.wrap(s.entOnly))
		{
			Template: "/quota/{specName}",
			Operations: []*operation{
				newOperation(http.MethodGet, "quotaSpecQuery", tags, "GetQuotaSpec",
					nil,
					append(queryOptions, &quotaSpecNameParam),
					newResponseConfig(200, objectSchema, api.QuotaSpec{}, queryMeta,
						"GetQuotaSpecResponse"),
				),
				newOperation(http.MethodPost, "quotaSpecUpdate", tags, "PostQuotaSpec",
					newRequestBody(objectSchema, api.QuotaSpec{}),
					append(writeOptions, &quotaSpecNameParam),
					newResponseConfig(200, nilSchema, nil, writeMeta,
						"PostQuotaSpecResponse"),
				),
				newOperation(http.MethodDelete, "quotaSpecDelete", tags, "DeleteQuotaSpec",
					nil,
					append(writeOptions, &quotaSpecNameParam),
					newResponseConfig(200, nilSchema, nil, writeMeta,
						"DeleteNamespaceResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/quota", s.wrap(s.entOnly))
		{
			Template: "/quota",
			Operations: []*operation{
				newOperation(http.MethodPost, "quotaSpecUpdate", tags, "CreateQuotaSpec",
					newRequestBody(objectSchema, api.QuotaSpec{}),
					writeOptions,
					newResponseConfig(200, nilSchema, nil, writeMeta,
						"CreateQuotaSpecResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/recommendation", s.wrap(s.entOnly))
		//s.mux.HandleFunc("/v1/recommendations", s.wrap(s.entOnly))
		//s.mux.HandleFunc("/v1/recommendations/apply", s.wrap(s.entOnly))
		//s.mux.HandleFunc("/v1/recommendation/", s.wrap(s.entOnly))
	}
}
