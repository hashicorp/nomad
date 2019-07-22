---
layout: "docs"
page_title: "Task Driver Plugins: Community Supported"
sidebar_current: "docs-drivers-community"
description: |-
  A list of community supported Task Driver Plugins.
---

# Community Supported

If you have authored a task driver plugin that you believe will be useful to the
broader Nomad community and you are committed to maintaining the plugin, please
file a PR to add your plugin to this page.

For details on authoring a task driver plugin, please refer to the [plugin
authoring guide][plugin_guide].

## Task Driver Plugins

Nomad has a plugin system for defining task drivers. External task driver
plugins will have the same user experience as built in drivers.

Below is a list of community-supported task drivers you can use with Nomad:

- [LXC][lxc]
- [Singularity][singularity]
- [Jail task driver][jail-task-driver]

[lxc]: /docs/drivers/external/lxc.html
[plugin_guide]: /docs/internals/plugins/index.html
[singularity]: /docs/drivers/external/singularity.html
[jail-task-driver]: /docs/drivers/external/jail-task-driver.html
