import { Factory, trait } from 'ember-cli-mirage';

import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  // Hidden property used to compute the Summary hash
  groupNames: [],

  JobID: '',
  namespace: null,
  ratio: '',

  withSummary: trait({
    Summary: function() {
      return this.groupNames.reduce((summary, group) => {
        if (this.ratio) {
          summary[group] = parseRatio(this.ratio);
        } else {
          summary[group] = {
            Queued: faker.random.number(10),
            Complete: faker.random.number(10),
            Failed: faker.random.number(10),
            Running: faker.random.number(10),
            Starting: faker.random.number(10),
            Lost: faker.random.number(10),
          };
        }
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

// ratioStr is a DSL that takes the form
//
// '\d+: ([QCFRSL] \d+ )+'
// ex. '15: Q 10 R 30 C 10'
//
// The leading number represents the total number of allocations.
// Each letter corresponds to a job status, each following number is a proportion
// The proportions are mapped according to the total number.
// The above example results in 3 queued, 9 running, and 3 completed allocations,
// totalling 15.
function parseRatio(ratioStr) {
  const isValid = /^\d+: ([QSRCFL] \d+ ?)+$/.test(ratioStr);
  if (!isValid) {
    throw new Error(`Invalid ratio string ${ratioStr} when attempting to create job-summary`);
  }

  const mapping = {
    Q: 'Queued',
    S: 'Starting',
    R: 'Running',
    C: 'Complete',
    F: 'Failed',
    L: 'Lost',
  };

  // Separate count from proportions
  let [count, pattern] = ratioStr.split(':');
  count = parseInt(count);

  // Parse the proportions pattern
  const patternParts = pattern.trim().split(' ');
  const proportions = {};
  let total = 0;

  patternParts.forEach((el, idx) => {
    // Elements alternate type and count
    if (['Q', 'S', 'R', 'C', 'F', 'L'].includes(el)) {
      proportions[mapping[el]] = parseInt(patternParts[idx + 1]);
    } else {
      total += parseInt(el);
    }
  });

  // Project the proportions to match the count
  const projectionRatio = count / total;
  Object.keys(proportions).forEach(key => {
    proportions[key] = Math.round(proportions[key] * projectionRatio);
  });

  // Fill out missing keys
  return Object.assign(
    {
      Queued: 0,
      Starting: 0,
      Running: 0,
      Complete: 0,
      Failed: 0,
      Lost: 0,
    },
    proportions
  );
}
