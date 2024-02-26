/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `main = rule { memory_under_limit and cpu_under_limit }

memory_under_limit = rule {
  all job.task_groups as tg {
    all tg.tasks as task {
      task.resources.memory < 1000
    }
  }
}

cpu_under_limit = rule {
  all job.task_groups as tg {
    all tg.tasks as task {
      task.resources.cpu < 1000
    }
  }
}
`;
