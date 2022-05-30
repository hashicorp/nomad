import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  id: () => faker.random.words(3).split(' ').join('/').toLowerCase(),
  path() {
    return this.id;
  },
  namespace: 'default',
  keyValues() {
    return [
      { key: faker.database.column(), value: faker.database.collation() },
      { key: faker.database.column(), value: faker.database.collation() },
      { key: faker.database.column(), value: faker.database.collation() },
      { key: faker.database.column(), value: faker.database.collation() },
      { key: faker.database.column(), value: faker.database.collation() },
    ];
  },
});
