---
layout: "guides"
page_title: "Accessing the Web UI"
sidebar_current: "guides-web-ui-access"
description: |-
  Learn how to access the Nomad Web UI from a browser or from the CLI.
---

# Accessing the Web UI

The Nomad Web UI is served alongside the API. If you visit the Nomad server address in a web
browser, you will be redirected to the Web UI, which is served under `/ui`. If you are unsure what
port the Nomad HTTP API is running on, try the default port: `4646`.

The first page you will see is a listing of all Jobs for the default namespace.

[![Jobs List][img-jobs-list]][img-jobs-list]

The entire Web UI sitemap is [documented as an API](/api/ui.html).

## Getting to the Web UI from the CLI

In order to make it as seamless as possible to jump between the CLI and UI, the Nomad CLI has a
[`ui` subcommand](/docs/commands/ui.html). This command can take any identifier and open the
appropriate web page.

**Open the UI directly to look at a job:**

```
$ nomad ui redis-job
http://127.0.0.1:4646/ui/jobs/redis-job
```

**Open the UI directly to look at an allocation:**

```
$ nomad ui d4005969
Opening URL "http://127.0.0.1:4646/ui/allocations/d4005969-b16f-10eb-4fe1-a5374986083d"
```

[img-jobs-list]: /assets/images/guide-ui-jobs-list.png
