import { test, moduleFor } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

moduleFor('adapter:job', 'Unit | Adapter | Job', {
  unit: true,
  needs: ['service:token', 'service:system', 'model:namespace', 'adapter:application'],
  beforeEach() {
    window.sessionStorage.clear();

    this.server = startMirage();
    this.server.create('node');
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
    ['/v1/namespaces', `/v1/job/${jobId}`, `/v1/job/${jobId}/summary`],
    'The three requests made are /namespaces, /job/:id, and /job/:id/summary'
  );
});

test('When there is no token set in the token service, no x-nomad-token header is set', function(
  assert
) {
  const { pretender } = this.server;
  const jobId = 'job-1';

  this.subject().findRecord(null, { modelName: 'job' }, jobId);

  assert.notOk(
    pretender.handledRequests.mapBy('requestHeaders').some(headers => headers['X-Nomad-Token']),
    'No token header present on either job request'
  );
});

test('When a token is set in the token service, then x-nomad-token header is set', function(
  assert
) {
  const { pretender } = this.server;
  const jobId = 'job-1';
  const secret = 'here is the secret';

  this.subject().set('token.secret', secret);
  this.subject().findRecord(null, { modelName: 'job' }, jobId);

  assert.ok(
    pretender.handledRequests
      .mapBy('requestHeaders')
      .every(headers => headers['X-Nomad-Token'] === secret),
    'The token header is present on both job requests'
  );
});
