# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

log_level = "WARN"

plugin "nomad-device-example" {
  "config" {
    "list_period" = "15s"
    "attribute_config" = [
      {
        attribute_name  = "type"
        attribute_type  = "string"
        attribute_value = "files"
      },
      {
        attribute_name  = "memory"
        attribute_type  = "int"
        attribute_value = "30"
        unit            = "GB"
      },
      {
        attribute_name  = "package"
        attribute_type  = "string"
        attribute_value = "standard"
      },
      {
        attribute_name  = "cool-attribute"
        attribute_type  = "string"
        attribute_value = "attribute-wearing-sunglasses"
      },
      {
        attribute_name  = "priority"
        attribute_type  = "string"
        attribute_value = "high"
      },
    ]

    "device_config" = [
      {
        id = "T100"
      },
      {
        id = "P1"
      },
      {
        id        = "T100"
        unhealthy = true
      },
      {
        id        = "P1"
        unhealthy = true
      },
    ]
  }
}
