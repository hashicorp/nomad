import { test, moduleFor } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

moduleFor('adapter:job', 'Unit | Adapter | Job', {
  unit: true,
  beforeEach() {
    this.server = startMirage();
    this.server.create('job', { id: 'job-1' });
  },
  afterEach() {
    this.server.shutdown();
  },
});

test('The job summary is stitched into the job request', function(assert) {
  const { pretender } = this.server;
  const jobId = 'job-1';

  this.subject().findRecord(null, { modelName: 'job' }, jobId);

  assert.deepEqual(
    pretender.handledRequests.mapBy('url'),
    [`/v1/job/${jobId}`, `/v1/job/${jobId}/summary`],
    'The two requests made are /job/:id and /job/:id/summary'
  );
});
