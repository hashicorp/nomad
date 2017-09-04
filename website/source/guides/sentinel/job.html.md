---
layout: "guides"
page_title: "Sentinel Job Object"
sidebar_current: "guides-sentinel-job"
description: |-
  Job objects can be introspected to apply fine grained Sentinel policies.
---

# Sentinel Job Objects

The `job` object is made available to policies in the `submit-job` scope automatically, without an explicit import.
The object roughly maps to the [JSON Specification of jobs](/api/json-jobs.html), but some paths differ for better readability in policies.

Below is a list of fields that may be available:

* `job.id`
* `job.parent_id`
* `job.region`
* `job.name`
* `job.type`
* `job.priority`
* `job.all_at_once`
* `job.datacenters`
* `job.constraints` - `nil` or list of constraint objects.
* `job.task_groups` - `nil` or list of task group objects.
* `job.periodic`
* `job.parameterized`
* `job.payload`
* `job.meta`

A constraint object has the following fields:

* `constraint.operand`
* `constraint.left_target`
* `constraint.right_target`
* `constraint.string`

A `job.periodic` object has the following fields:

* `job.periodic.spec`
* `job.periodic.spec_type`
* `job.periodic.prohibit_overlap`
* `job.periodic.timezone`

A `job.parameterized` object has the following fields:

* `job.parameterized.payload_type`
* `job.parameterized.meta_required`
* `job.parameterized.meta_optional`

A task group object has the following fields:

* `task_group.name`
* `task_group.count`
* `task_group.update`
* `task_group.constraints` - `nil` or list of constraint objects.
* `task_group.restart_policy`
* `task_group.tasks`  - `nil` or list of task objects
* `task_group.ephemeral_disk`
* `task_group.meta`

A task group update object has the following fields:

* `task_group.update.stagger`
* `task_group.update.max_parallel`
* `task_group.update.health_check`
* `task_group.update.min_healthy_time`
* `task_group.update.healthy_deadline`
* `task_group.update.auto_revert`
* `task_group.update.canary`

A task group restart policy has the following fields:

* `task_group.restart.attempts`
* `task_group.restart.interval`
* `task_group.restart.delay`
* `task_group.restart.mode`

A task group ephemeral disk has the following fields:

* `task_group.ephemeral_disk.sticky`
* `task_group.ephemeral_disk.size_mb`
* `task_group.ephemeral_disk.migrate`

A task object has the following fields:

* `task.name`
* `task.driver`
* `task.config`
* `task.user`
* `task.env`
* `task.services` - `nil` or list of service objects.
* `task.vault`
* `task.templates`
* `task.constraints` - `nil` or list of constraint objects.
* `task.resources`
* `task.dispatch_payload`
    * `task.dispatch_payload.file`
* `task.meta`
* `task.kill_timeout`
* `task.log_config`
    * `task.log_config.max_files`
    * `task.log_config.max_filesize_mb`
* `task.artifacts`
* `task.leader`

A task service object has the following fields:

* `service.name`
* `service.port_label`
* `service.address_mode`
* `service.tags`
* `service.checks`

A service check object has the following fields:

* `check.name`
* `check.type`
* `check.command`
* `check.args`
* `check.path`
* `check.protocol`
* `check.port_label`
* `check.interval`
* `check.timeout`
* `check.initial_status`
* `check.tls_skip_verify`

A task Vault object has the following fields:

* `task.vault.policies`
* `task.vault.env`
* `task.vault.change_mode`
* `task.vault.change_signal`

A task template object has the following fields:

* `template.source_path`
* `template.destination_path`
* `template.embedded_template`
* `template.change_mode`
* `template.change_signal`
* `template.splay`
* `template.permissions`
* `template.left_delimiter`
* `template.right_delimiter`
* `template.env_vars`

A task resource object has the following fields:

* `task.resources.cpu`
* `task.resources.memory_mb`
* `task.resources.disk_mb`
* `task.resources.iops`
* `task.resources.networks`

A network resource object has the following fields:

* `network.device`
* `network.cidr`
* `network.ip`
* `network.mbits`
* `network.reserved_ports`
* `network.dynamic_ports`

A network port object has the following fields:

* `port.label`
* `port.value`

A task artifact object has the following fields:

* `artifact.source`
* `artifact.options`
* `artifact.getter_mode`
* `artifact.relative_destination`

