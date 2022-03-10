# Carbon-aware Nomad Experiment

This branch is an experiment to enable Nomad to minimize the climate impact of
the compute it manages. In particular it takes the carbon impact of nodes into
account when scheduling: prioritizing the use of of lower-carbon-producing
compute.

## Example

```
$ nomad agent -dev -config carbon.hcl  # see below for hcl

# In another terminal
$ nomad init
$ nomad run example.nomad
$ nomad alloc status -verbose <alloc id here>
ID                  = 144a2dfc-45de-6e2f-74f0-7cc885c2c846
Eval ID             = d04918d7-02f0-6f64-f46f-0995fa012c2e
Name                = example.cache[0]
...

Placement Metrics
Weights:  0.5      1.5     2                  1              1                        final
Node      binpack  carbon  job-anti-affinity  node-affinity  node-reschedule-penalty  score
84176148  0.0391   -0.5    0                  0              0                        -0.365
```

## Changes

### Scheduling

**Carbon scoring is DISABLED by default. Use the HCL or JSON below to enable.**

A new Carbon Scoring algorithm has been added to the scheduler. When enabled
the higher a node's carbon score, the less likely it will receive work.

Scoring weighting has also been added. To enable carbon scoring you must give
it a non-zero weight either on startup with the following server config:

```hcl
server {
  default_scheduler_config {
    # Set a default carbon score in case not all nodes fingerprint it
    carbon_default_score = 50

    # These weights will enable carbon scoring and prioritize it over
    # binpacking but less than job-anti-affinity
    scoring_weights = {
      "job-anti-affinity" = 2
      carbon              = 1.5
      binpack             = 0.5
    }
  }
}
```

Or on a live cluster by updating the cluster scheduling configuration:

```json
  {
    "CarbonDefaultScore": 50,
    "CreateIndex": 5,
    "MemoryOversubscriptionEnabled": false,
    "ModifyIndex": 5,
    "PreemptionConfig": {
      "BatchSchedulerEnabled": false,
      "ServiceSchedulerEnabled": false,
      "SysBatchSchedulerEnabled": false,
      "SystemSchedulerEnabled": false
    },
    "RejectJobRegistration": false,
    "SchedulerAlgorithm": "binpack",
    "ScoringWeights": {
      "job-anti-affinity": 2,
      "carbon": 1.5,
      "binpack": 0.5
    }
  }
```

If that json is in the file `conf.json`:

```shell-session
$ nomad operator api -X POST /v1/operator/scheduler/configuration < conf.json
{"Index":21,"Updated":true}
```

### Nodes

Nodes now fingerprint their carbon usage.
`Node.Attributes["energy.carbon_score"]` is a number between 1-100 with 1 being
the lowest carbon and 100 being the highest.

## Resources

* https://www.cloudcarbonfootprint.org/
* https://electricitymap.org/
