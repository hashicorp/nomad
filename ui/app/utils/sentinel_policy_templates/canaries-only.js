/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `# This policy ensures that all deployments must use canary deployments.

canary_required = rule {
  all job.task_groups as tg {
    tg.update.canary > 0
  }
}

main = rule { canary_required }`;
