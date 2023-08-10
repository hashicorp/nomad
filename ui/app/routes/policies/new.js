/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

const INITIAL_POLICY_RULES = `# See https://developer.hashicorp.com/nomad/tutorials/access-control/access-control-policies for ACL Policy details

# Example policy structure:

namespace "default" {
  policy = "deny"
  capabilities = []
}

namespace "example-ns" {
  policy = "deny"
  capabilities = ["list-jobs", "read-job"]
  variables {
    # list access to variables in all paths, full access in nested/variables/*
    path "*" {
      capabilities = ["list"]
    }
    path "nested/variables/*" {
      capabilities = ["write", "read", "destroy", "list"]
    }
  }
}

host_volume "example-volume" {
  policy = "deny"
}

agent {
  policy = "deny"
}

node {
  policy = "deny"
}

quota {
  policy = "deny"
}

operator {
  policy = "deny"
}

# Possible Namespace Policies:
#  * deny
#  * read
#  * write
#  * scale

# Possible Namespace Capabilities:
#  * list-jobs
#  * parse-job
#  * read-job
#  * submit-job
#  * dispatch-job
#  * read-logs
#  * read-fs
#  * alloc-exec
#  * alloc-lifecycle
#  * csi-write-volume
#  * csi-mount-volume
#  * list-scaling-policies
#  * read-scaling-policy
#  * read-job-scaling
#  * scale-job

# Possible Variables capabilities
#  * write
#  * read
#  * destroy
#  * list

# Possible Policies for "agent", "node", "quota", "operator", and "host_volume":
#  * deny
#  * read
#  * write
`;

export default class PoliciesNewRoute extends Route {
  @service can;
  @service router;

  beforeModel() {
    if (this.can.cannot('write policy')) {
      this.router.transitionTo('/policies');
    }
  }

  model() {
    return this.store.createRecord('policy', {
      name: '',
      rules: INITIAL_POLICY_RULES,
    });
  }

  resetController(controller, isExiting) {
    // If the user navigates away from /new, clear the path
    controller.set('path', null);
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller.model.isNew) {
        controller.model.destroyRecord();
      }
    }
  }
}
