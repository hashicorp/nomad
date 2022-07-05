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

    // Create a special policy with secure variables rules in place
    if (token.id === '53cur3-v4r14bl35') {
      const variableMakerPolicy = {
        id: 'Variable Maker',
        rules: `
# Allow read only access to the default namespace
namespace "default" {
  policy = "read"
  capabilities = ["list-jobs", "alloc-exec", "read-logs"]
  secure_variables {
    # full access to secrets in all project paths
    path "blue/*" {
      capabilities = ["write", "read", "destroy", "list"]
    }

    # full access to secrets in all project paths
    path "*" {
      capabilities = ["write", "read", "destroy", "list"]
    }

    # read/list access within a "system" path belonging to administrators
    path "system/*" {
      capabilities = ["read", "list"]
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
              Name: 'default',
              Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
              SecureVariables: {
                Paths: [
                  {
                    Capabilities: ['write', 'read', 'destroy', 'list'],
                    PathSpec: 'blue/*',
                  },
                  {
                    Capabilities: ['write', 'read', 'destroy', 'list'],
                    PathSpec: '*',
                  },
                  {
                    Capabilities: ['read', 'list'],
                    PathSpec: 'system/*',
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
  },
});
