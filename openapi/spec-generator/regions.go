package main

import (
	"net/http"
)

func (v *v1api) getRegionsPaths() []*Path {
	tags := []string{"Regions"}

	return []*Path{
		//s.mux.HandleFunc("/v1/regions", s.wrap(s.RegionListRequest))
		{
			Template: "/regions",
			Operations: []*Operation{
				newOperation(http.MethodGet, "regionListRequest", tags, "GetRegions",
					nil,
					nil,
					newResponseConfig(200, arraySchema, "string", nil, "GetRegionsResponse"),
				),
			},
		},
	}
}
