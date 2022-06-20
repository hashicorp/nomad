import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  id: () => faker.random.words(3).split(' ').join('/').toLowerCase(),
  path() {
    return this.id;
  },
  namespace: 'default',
  createdIndex: 100,
  modifiedIndex: 100,
  createTime: () => faker.date.past(15),
  modifyTime: () => faker.date.recent(1),
  items() {
    return (
      this.Items || {
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
      }
    );
  },
});
