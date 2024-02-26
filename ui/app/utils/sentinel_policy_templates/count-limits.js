/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `main = rule { all_counts_under }

# all_counts_under checks that all task group counts are under a certain value

all_counts_under = rule {
  all job.task_groups as tg {
    all tg.count as count {
      count < 100
    }
  }
}
`;
