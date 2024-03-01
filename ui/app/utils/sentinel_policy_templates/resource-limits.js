/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `import "units"

resource_check = func(task_groups, resource) {
  result = 0
  for task_groups as g {
    for g.tasks as t {
        result = result + t.resources[resource] * g.count
    }
  }
  return result
}

main = rule  {
  resource_check(job.task_groups, "cpu") <= 1500 and
  resource_check(job.task_groups, "memory_mb") <= 2500
}`;
