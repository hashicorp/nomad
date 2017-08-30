import { Factory, faker } from 'ember-cli-mirage';

const REF_TIME = new Date();

export default Factory.extend({
  stable: faker.random.boolean,
  submitTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,
  diff: () => generateDiff(),

  jobId: null,
  version: 0,
});

function generateDiff() {
  return {};
}
