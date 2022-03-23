# Unfortunately 'variable' interpolation isn't supported in volume spec
# so, parameters has to be supplied again

id = "csi-test"

name = "csi-test"

type = "csi"

plugin_id = "kadalu-csi"

capacity_min = "200M"

capacity_max = "1G"

capability {
  access_mode     = "multi-node-multi-writer"
  attachment_mode = "file-system"
}

parameters {
  kadalu_format = "native"

  # Below parameters needs to be replaced correctly based on
  # json file supplied during controller/nodeplugin job
  storage_name = "POOL"

  gluster_hosts   = "GHOST"
  gluster_volname = "GVOL"
}
