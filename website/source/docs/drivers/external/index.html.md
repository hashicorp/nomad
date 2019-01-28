---
layout: "docs"
page_title: "External Plugins"
sidebar_current: "docs-external-plugins"
description: |-
  External plugins allow you easily extend Nomad's functionality and further
  support customized workloads.
---

# External Plugins

Starting with Nomad 0.9, task and device drivers are now pluggable. This gives users the flexibility to introduce their own drivers without having to recompile Nomad. You can view the [plugin stanza][plugin] documentation for examples on how to use the `plugin` stanza in Nomad's client configuration. 

Below is a list of external drivers you can use with Nomad:

- [LXC][lxc]

[lxc]: /docs/drivers/external/lxc.html 
[plugin]: /docs/configuration/plugin.html
