---
layout: "docs"
page_title: "Commands: init"
sidebar_current: "docs-commands-init"
description: >
  Generate a skeleton jobspec template.
---

# Command: init

The `init` command creates an example [job specification][jobspec] in the
current directory that demonstrates some common configurations for tasks, task
groups, runtime constraints, and resource allocation.

Please refer to the [jobspec][] and [drivers](/docs/drivers/index.html)
pages to learn how to customize the template.

## Examples

Generate an example job file:

```text
$ nomad init
Example job file written to example.nomad
```

[jobspec]: /docs/job-specification/index.html "Nomad Job Specification"
