# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

id           = "postgresql-database"
name         = "postgresql-database"
type         = "csi"
plugin_id    = "juicefs"
capacity_min = "5GiB"
capacity_max = "5GiB"

capability {
  access_mode = "multi-node-multi-writer"
  attachment_mode = "file-system"
}

mount_options {
  fs_type     = "ext4"
  # Uncomment the following to have the volume registered with Consul and allow metrics to be discoverable
  #mount_flags = ["consul=127.0.0.1:8500","metrics=0.0.0.0:0"]
}

secrets {
  # For community edition, use the following:
  name="postgresql-database"

  # Example using TiKV for metadata storage, adjust accordingly for your environment
  metaurl="tikv://tikv.service.example.org:2379/production/postgresql-database"

  # Data storage using Minio, adjust accordingly for your environment
  storage="minio"
  bucket="http://minio.example.org:9000/production"
  access-key="administrator"
  secret-key="dead-beef"

  # Uncomment to disable trash
  #trash-days="0"
}
