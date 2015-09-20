---
layout: "docs"
page_title: "Telemetry"
sidebar_current: "docs-internals-telemetry"
description: |-
  Learn about the telemetry data available in Nomad.
---

# Telemetry

The Nomad agent collects various runtime metrics about the performance of
different libraries and subsystems. These metrics are aggregated on a ten
second interval and are retained for one minute.

To view this data, you must send a signal to the Nomad process: on Unix,
this is `USR1` while on Windows it is `BREAK`. Once Nomad receives the signal,
it will dump the current telemetry information to the agent's `stderr`.

This telemetry information can be used for debugging or otherwise
getting a better view of what Nomad is doing.

Telemetry information can be streamed to both [statsite](http://github.com/armon/statsite)
as well as statsd based on providing the appropriate configuration options.

Below is sample output of a telemetry dump:

```text
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.nomad.broker.total_blocked': 0.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.nomad.plan.queue_depth': 0.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.malloc_count': 7568.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.total_gc_runs': 8.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.nomad.broker.total_ready': 0.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.num_goroutines': 56.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.sys_bytes': 3999992.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.heap_objects': 4135.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.nomad.heartbeat.active': 1.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.nomad.broker.total_unacked': 0.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.nomad.broker.total_waiting': 0.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.alloc_bytes': 634056.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.free_count': 3433.000
[2015-09-17 16:59:40 -0700 PDT][G] 'nomad.runtime.total_gc_pause_ns': 6572135.000
[2015-09-17 16:59:40 -0700 PDT][C] 'nomad.memberlist.msg.alive': Count: 1 Sum: 1.000
[2015-09-17 16:59:40 -0700 PDT][C] 'nomad.serf.member.join': Count: 1 Sum: 1.000
[2015-09-17 16:59:40 -0700 PDT][C] 'nomad.raft.barrier': Count: 1 Sum: 1.000
[2015-09-17 16:59:40 -0700 PDT][C] 'nomad.raft.apply': Count: 1 Sum: 1.000
[2015-09-17 16:59:40 -0700 PDT][C] 'nomad.nomad.rpc.query': Count: 2 Sum: 2.000
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.serf.queue.Query': Count: 6 Sum: 0.000
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.fsm.register_node': Count: 1 Sum: 1.296
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.serf.queue.Intent': Count: 6 Sum: 0.000
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.runtime.gc_pause_ns': Count: 8 Min: 126492.000 Mean: 821516.875 Max: 3126670.000 Stddev: 1139250.294 Sum: 6572135.000
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.raft.leader.dispatchLog': Count: 3 Min: 0.007 Mean: 0.018 Max: 0.039 Stddev: 0.018 Sum: 0.054
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.leader.reconcileMember': Count: 1 Sum: 0.007
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.leader.reconcile': Count: 1 Sum: 0.025
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.raft.fsm.apply': Count: 1 Sum: 1.306
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.client.get_allocs': Count: 1 Sum: 0.110
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.worker.dequeue_eval': Count: 29 Min: 0.003 Mean: 363.426 Max: 503.377 Stddev: 228.126 Sum: 10539.354
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.serf.queue.Event': Count: 6 Sum: 0.000
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.raft.commitTime': Count: 3 Min: 0.013 Mean: 0.037 Max: 0.079 Stddev: 0.037 Sum: 0.110
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.leader.barrier': Count: 1 Sum: 0.071
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.client.register': Count: 1 Sum: 1.626
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.nomad.eval.dequeue': Count: 21 Min: 500.610 Mean: 501.753 Max: 503.361 Stddev: 1.030 Sum: 10536.813
[2015-09-17 16:59:40 -0700 PDT][S] 'nomad.memberlist.gossip': Count: 12 Min: 0.009 Mean: 0.017 Max: 0.025 Stddev: 0.005 Sum: 0.204
```
