# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "example" {
  group "group" {
    task "task" {
      config {
        ## arbitrary structures to compare HCLv1 and HCLv2
        ## HCLv2 must parse the same way HCLv1 parser does


        # primitive attrs
        attr_string    = "b"
        attr_int       = 3
        attr_large_int = 21474836470
        attr_float     = 3.2

        # lists attrs
        attr_list_string = ["a", "b"]
        attr_list_int    = [1, 2, 4]
        attr_list_float  = [1.2, 2.3, 4.2]
        attr_list_hetro  = [1, "a", 3.4, { "k" = "v" }]
        attr_list_empty  = []

        # map attrs
        attr_map       = { "KEY" = "VAL", "KEY2" = "VAL2" }
        attr_map_empty = {}

        # simple blocks
        block1 {
          k    = "b"
          key2 = "v2"
        }
        labeled_block "label1" {
          k = "b"
        }
        multi_labeled_block "label1" "label2" "label3" {
          k = "b"
        }

        # repeated block
        repeated_block_type {
          a = 2
        }
        repeated_block_type {
          b = 3
        }

        # repeated blocks with labels
        label_repeated "l1" {
          a = 1
        }
        label_repeated "l1" {
          a = 2
        }
        label_repeated "l2" "l21" {
          a = 3
        }
        label_repeated "l2" "l21" "l23" {
          a = 4
        }
        label_repeated "l3" {
          a = 5
        }

        # nested blocks
        outer_block "l1" {
          level = 1
          inner_block "l2" {
            level = 2
            most_inner "l3" {
              level = 3

              inner_map = { "K" = "V" }
            }
          }
        }
      }
    }
  }
}
