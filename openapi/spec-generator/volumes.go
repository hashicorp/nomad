package main

import (
	"github.com/hashicorp/nomad/api"
	"net/http"
)

func (v *v1api) getVolumesPaths() []*Path {
	tags := []string{"Volumes"}

	return []*Path{
		//s.mux.HandleFunc("/v1/volumes", s.wrap(s.CSIVolumesRequest))
		{
			Template: "/volumes",
			Operations: []*Operation{
				newOperation(http.MethodGet, "CSIVolumesRequest", tags, "GetVolumes",
					nil,
					newGetVolumesParameters(),
					newResponseConfig(200, arraySchema, api.CSIVolumeListStub{}, queryMeta, "GetVolumesResponse"),
				),
				newOperation(http.MethodPost, "csiVolumeRegister", tags, "PostVolume",
					newRequestBody(objectSchema, api.CSIVolumeRegisterRequest{}),
					writeOptions,
					newResponseConfig(200, "", nil, queryMeta, "PostVolumeResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/volumes/external", s.wrap(s.CSIExternalVolumesRequest))
		{
			Template: "/volumes/external",
			Operations: []*Operation{
				newOperation(http.MethodGet, "CSIExternalVolumesRequest", tags, "GetExternalVolumes",
					nil,
					append(queryOptions, &VolumePluginIDParam),
					newResponseConfig(200, objectSchema, api.CSIVolumeListExternalResponse{}, queryMeta, "GetExternalVolumesResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/volumes/snapshot", s.wrap(s.CSISnapshotsRequest))
		{
			Template: "/volumes/snapshot",
			Operations: []*Operation{
				newOperation(http.MethodGet, "csiSnapshotList", tags, "GetSnapshots",
					nil,
					append(queryOptions, &VolumePluginIDParam),
					newResponseConfig(200, objectSchema, api.CSISnapshotListResponse{}, queryMeta,
						"GetSnapshotsResponse"),
				),
				// TODO: See if there is a way to override mismatch between the naming convention and the struct name.
				newOperation(http.MethodPost, "csiSnapshotCreate", tags, "PostSnapshot",
					newRequestBody(objectSchema, api.CSISnapshotCreateRequest{}),
					writeOptions,
					newResponseConfig(200, objectSchema, api.CSISnapshotCreateResponse{}, writeMeta,
						"PostSnapshotResponse"),
				),
				newOperation(http.MethodDelete, "csiSnapshotDelete", tags, "DeleteSnapshot",
					nil,
					newDeleteSnapshotParameters(),
					newResponseConfig(200, "", nil, writeMeta,
						"DeleteSnapshotResponse"),
				),
			},
		},
		//s.mux.HandleFunc("/v1/volume/csi/", s.wrap(s.CSIVolumeSpecificRequest))
		{
			Left off here
			Template: "/volume/csi/{volumeId}",
			Operations: []*Operation{
				newOperation(http.MethodGet, "csiSnapshotList", tags, "GetSnapshots",
					nil,
					append(queryOptions, &VolumePluginIDParam),
					newResponseConfig(200, objectSchema, api.CSISnapshotListResponse{}, queryMeta,
						"GetSnapshotsResponse"),
				),
				// TODO: See if there is a way to override mismatch between the naming convention and the struct name.
				newOperation(http.MethodPost, "csiSnapshotCreate", tags, "PostSnapshot",
					newRequestBody(objectSchema, api.CSISnapshotCreateRequest{}),
					writeOptions,
					newResponseConfig(200, objectSchema, api.CSISnapshotCreateResponse{}, writeMeta,
						"PostSnapshotResponse"),
				),
				newOperation(http.MethodDelete, "csiSnapshotDelete", tags, "DeleteSnapshot",
					nil,
					newDeleteSnapshotParameters(),
					newResponseConfig(200, "", nil, writeMeta,
						"DeleteSnapshotResponse"),
				),
			},
		},
	}
}

func newGetVolumesParameters() []*Parameter {
	params := append(queryOptions, &VolumeNodeIDParam)
	params = append(params, &VolumePluginIDParam)
	params = append(params, &VolumeTypeParam)

	return params
}

func newDeleteSnapshotParameters() []*Parameter {
	params := append(writeOptions, &VolumePluginIDParam)
	params = append(params, &SnapshotIDParam)

	return params
}
