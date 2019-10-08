import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  id: () => faker.random.uuid(),
  accessorId() {
    return this.id;
  },
  secretId: () => faker.random.uuid(),
  name: () => faker.name.findName(),
  global: () => faker.random.boolean(),
  type: i => (i === 0 ? 'management' : 'client'),

  afterCreate(token, server) {
    const policyIds = Array(faker.random.number({ min: 1, max: 5 }))
      .fill(0)
      .map(() => faker.hacker.verb())
      .uniq();

    policyIds.forEach(policy => {
      const dbPolicy = server.db.policies.find(policy);
      if (!dbPolicy) {
        server.create('policy', { id: policy });
      }
    });

    token.update({ policyIds });
  },
});
