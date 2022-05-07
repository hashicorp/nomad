import { Factory, trait } from 'ember-cli-mirage';

import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  // Hidden property used to compute the Summary hash
  groupNames: [],

  namespace: null,

  withSummary: trait({
    Summary: function () {
      return this.groupNames.reduce((summary, group) => {
        summary[group] = {
          Queued: faker.random.number(10),
          Complete: faker.random.number(10),
          Failed: faker.random.number(10),
          Running: faker.random.number(10),
          Starting: faker.random.number(10),
          Lost: faker.random.number(10),
          Unknown: faker.random.number(10),
        };
        return summary;
      }, {});
    },
  }),

  withChildren: trait({
    Children: () => ({
      Pending: faker.random.number(10),
      Running: faker.random.number(10),
      Dead: faker.random.number(10),
    }),
  }),
});
