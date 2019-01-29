---
layout: "guides"
page_title: "Sentinel Job Object"
sidebar_current: "guides-security-sentinel-job"
description: |-
  Job objects can be introspected to apply fine grained Sentinel policies.
---

# Sentinel Job Objects

The `job` object is made available to policies in the `submit-job` scope automatically, without an explicit import.
The object maps to the [JSON Specification of jobs](/api/json-jobs.html), but fields differ slightly for better readability.

Sentinel convention for identifiers is lower case and separated by underscores. All fields on the job are accessed by the same name, converted to lower case and separating camel case to underscores. Here are some examples:

| Job Field                               | Sentinel Accessor      |
| --------------------------------------- | ---------------------- |
| `job.ID       `                         | `job.id`               |
| `job.AllAtOnce`                         | `job.all_at_once`      |
| `job.ParentID`                          | `job.parent_id`        |
| `job.TaskGroups`                        | `job.task_groups`      |
| `job.TaskGroups[0].EphemeralDisk.SizeMB`| `job.task_groups[0].ephemeral_disk.size_mb` |

