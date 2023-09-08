/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  id: () => faker.random.uuid(),
  accessorId() {
    return this.id;
  },
  secretId: () => faker.random.uuid(),
  name: (i) => `${i === 0 ? 'Manager ' : ''}${faker.name.findName()}`,
  global: () => faker.random.boolean(),
  type: (i) => (i === 0 ? 'management' : 'client'),

  oneTimeSecret: () => faker.random.uuid(),

  afterCreate(token, server) {
    if (token.policyIds && token.policyIds.length) return;
    const policyIds = Array(faker.random.number({ min: 1, max: 5 }))
      .fill(0)
      .map(() => faker.hacker.verb())
      .uniq();

    policyIds.forEach((policy) => {
      const dbPolicy = server.db.policies.find(policy);
      if (!dbPolicy) {
        server.create('policy', { id: policy });
      }
    });

    token.update({ policyIds });

    // Create a special policy with variables rules in place
    if (token.id === '53cur3-v4r14bl35') {
      const variableMakerPolicy = {
        id: 'Variable Maker',
        rules: `
# Allow read only access to the default namespace
namespace "*" {
  policy = "read"
  capabilities = ["list-jobs", "alloc-exec", "read-logs"]
  variables {
    # Base access is to all abilities for all variables
    path "*" {
      capabilities = ["list", "read", "destroy", "create"]
    }
  }
}

node {
  policy = "read"
}
      `,

        rulesJSON: {
          Namespaces: [
            {
              Name: '*',
              Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
              Variables: {
                Paths: [
                  {
                    Capabilities: ['write', 'read', 'destroy', 'list'],
                    PathSpec: '*',
                  },
                ],
              },
            },
          ],
        },
      };
      server.create('policy', variableMakerPolicy);
      token.policyIds.push(variableMakerPolicy.id);
    }
    if (token.id === 'f3w3r-53cur3-v4r14bl35') {
      const variableViewerPolicy = {
        id: 'Variable Viewer',
        rules: `
# Allow read only access to the default namespace
namespace "*" {
  policy = "read"
  capabilities = ["list-jobs", "alloc-exec", "read-logs"]
  variables {
    # Base access is to all abilities for all variables
    path "*" {
      capabilities = ["list"]
    }
  }
}

namespace "namespace-1" {
  policy = "read"
  capabilities = ["list-jobs", "alloc-exec", "read-logs"]
  variables {
    # Base access is to all abilities for all variables
    path "*" {
      capabilities = ["list", "read", "destroy", "create"]
    }
  }
}

namespace "namespace-2" {
  policy = "read"
  capabilities = ["list-jobs", "alloc-exec", "read-logs"]
  variables {
    # Base access is to all abilities for all variables
    path "blue/*" {
      capabilities = ["list", "read", "destroy", "create"]
    }
    path "nomad/jobs/*" {
      capabilities = ["list", "read", "create"]
    }
  }
}

node {
  policy = "read"
}
      `,

        rulesJSON: {
          Namespaces: [
            {
              Name: '*',
              Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
              Variables: {
                Paths: [
                  {
                    Capabilities: ['list'],
                    PathSpec: '*',
                  },
                ],
              },
            },
            {
              Name: 'namespace-1',
              Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
              Variables: {
                Paths: [
                  {
                    Capabilities: ['list', 'read', 'destroy', 'create'],
                    PathSpec: '*',
                  },
                ],
              },
            },
            {
              Name: 'namespace-2',
              Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
              Variables: {
                Paths: [
                  {
                    Capabilities: ['list', 'read', 'destroy', 'create'],
                    PathSpec: 'blue/*',
                  },
                  {
                    Capabilities: ['list', 'read', 'create'],
                    PathSpec: 'nomad/jobs/*',
                  },
                ],
              },
            },
          ],
        },
      };
      server.create('policy', variableViewerPolicy);
      token.policyIds.push(variableViewerPolicy.id);
    }
    if (token.id === '3XP1R35-1N-3L3V3N-M1NU735') {
      token.update({
        expirationTime: new Date(new Date().getTime() + 11 * 60 * 1000),
      });
    }
  },
});
