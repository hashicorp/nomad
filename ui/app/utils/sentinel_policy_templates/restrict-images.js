/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `# This policy restricts which Docker images from which Docker registries are
# allowed and also prevents use of the "latest" tag to ensure predictability

import "strings"

allowed_registries = [
   "https://hub.docker.internal",
]

# Allowed Docker images
allowed_images = [
  "nginx",
  "mongo",
]

check_task_config = func(task) {
  	status = true
    registry = "hub.docker.io"
    image = ""
    if task.driver in ["docker", "podman"] {
      registry_and_image = strings.split(task.config.image, ("/"))
      if length(registry_and_image) > 1 {
         registry = registry_and_image[0]
         image = registry_and_image[1]
      } else {
         image = task.config.image
      }
      # Checking the image
	    for allowed_images as allowed {
        # Check for allowed images
    	  if (!strings.has_prefix(image, allowed + ":")) {
          print(task.config.image, "in task", task.name, "does not conform to policy, not in allowed images", allowed_images)
          status = false
        } else {
          status = true
          break
        }
      }
      # Check for latest
    	if (strings.has_suffix(image, ":latest")) {
          print(task.config.image, "in task", task.name, "does not conform to policy, using :latest instead of a specific version")
          status = false
      }
      # Check registry
      if registry not in allowed_registries {
        print(task.config.image, "in task", task.name, "does not conform to policy, not from an allowed registry", allowed_registries)
        status = false
      }
      return status
    }
}

# Restrict allowed Docker images
restrict_images = rule {
  all job.task_groups as tg {
    all tg.tasks as task {
      check_task_config(task)
    }
  }
}

# Main rule
main = rule {
  restrict_images
}`;
