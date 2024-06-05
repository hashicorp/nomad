/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `# This policy restricts which Docker images are allowed and also prevents use of
# the "latest" tag since the image must specify a tag that starts with a number.

# Allowed Docker images
allowed_images = [
  "https://hub.docker.internal",
  "nginx",
  "mongo",
]

# Restrict allowed Docker images
restrict_images = rule {
  all job.task_groups as tg {
    all tg.tasks as task {
      any allowed_images as allowed {
        # Note that we require ":" and a tag after it
        # which must start with a number, preventing "latest"
        task.config.image matches allowed + ":[0-9](.*)"
      }
    }
  }
}

# Main rule
main = rule {
  restrict_images
}`;
