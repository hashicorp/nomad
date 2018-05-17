import { run } from '@ember/runloop';
import { assign } from '@ember/polyfills';
import { test } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import moduleForAdapter from '../../helpers/module-for-adapter';

moduleForAdapter('job', 'Unit | Adapter | Job', {
  needs: [
    'adapter:job',
    'service:token',
    'service:system',
    'model:namespace',
    'model:job-summary',
    'adapter:application',
    'service:watchList',
  ],
  beforeEach() {
    window.sessionStorage.clear();

    this.server = startMirage();
    this.server.create('node');
    this.server.create('job', { id: 'job-1' });
    this.server.create('job', { id: 'job-2', namespaceId: 'some-namespace' });
  },
  afterEach() {
    this.server.shutdown();
  },
});

test('The job summary is stitched into the job request', function(assert) {
  const { pretender } = this.server;
  const jobName = 'job-1';
  const jobNamespace = 'default';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  this.subject().findRecord(null, { modelName: 'job' }, jobId);

  assert.deepEqual(
    pretender.handledRequests.mapBy('url'),
    ['/v1/namespaces', `/v1/job/${jobName}`],
    'The two requests made are /namespaces and /job/:id'
  );
});

test('When the job has a namespace other than default, it is in the URL', function(assert) {
  const { pretender } = this.server;
  const jobName = 'job-2';
  const jobNamespace = 'some-namespace';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  this.subject().findRecord(null, { modelName: 'job' }, jobId);

  assert.deepEqual(
    pretender.handledRequests.mapBy('url'),
    ['/v1/namespaces', `/v1/job/${jobName}?namespace=${jobNamespace}`],
    'The two requests made are /namespaces and /job/:id?namespace=:namespace'
  );
});

test('When there is no token set in the token service, no x-nomad-token header is set', function(assert) {
  const { pretender } = this.server;
  const jobId = JSON.stringify(['job-1', 'default']);

  this.subject().findRecord(null, { modelName: 'job' }, jobId);

  assert.notOk(
    pretender.handledRequests.mapBy('requestHeaders').some(headers => headers['X-Nomad-Token']),
    'No token header present on either job request'
  );
});

test('When a token is set in the token service, then x-nomad-token header is set', function(assert) {
  const { pretender } = this.server;
  const jobId = JSON.stringify(['job-1', 'default']);
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

test('findAll can be watched', function(assert) {
  const { pretender } = this.server;

  const request = () =>
    this.subject().findAll(null, { modelName: 'job' }, null, {
      reload: true,
      adapterOptions: { watch: true },
    });

  request();
  assert.equal(
    pretender.handledRequests[0].url,
    '/v1/namespaces',
    'First request is for namespaces'
  );
  assert.equal(
    pretender.handledRequests[1].url,
    '/v1/jobs?index=1',
    'Second request is a blocking request for jobs'
  );

  return wait().then(() => {
    request();
    assert.equal(
      pretender.handledRequests[2].url,
      '/v1/jobs?index=2',
      'Third request is a blocking request with an incremented index param'
    );

    return wait();
  });
});

test('findRecord can be watched', function(assert) {
  const jobId = JSON.stringify(['job-1', 'default']);
  const { pretender } = this.server;

  const request = () =>
    this.subject().findRecord(null, { modelName: 'job' }, jobId, {
      reload: true,
      adapterOptions: { watch: true },
    });

  request();
  assert.equal(
    pretender.handledRequests[0].url,
    '/v1/namespaces',
    'First request is for namespaces'
  );
  assert.equal(
    pretender.handledRequests[1].url,
    '/v1/job/job-1?index=1',
    'Second request is a blocking request for job-1'
  );

  return wait().then(() => {
    request();
    assert.equal(
      pretender.handledRequests[2].url,
      '/v1/job/job-1?index=2',
      'Third request is a blocking request with an incremented index param'
    );

    return wait();
  });
});

test('relationships can be reloaded', function(assert) {
  const { pretender } = this.server;
  const plainId = 'job-1';
  const mockModel = makeMockModel(plainId);

  this.subject().reloadRelationship(mockModel, 'summary');
  assert.equal(
    pretender.handledRequests[0].url,
    `/v1/job/${plainId}/summary`,
    'Relationship was reloaded'
  );
});

test('relationship reloads can be watched', function(assert) {
  const { pretender } = this.server;
  const plainId = 'job-1';
  const mockModel = makeMockModel(plainId);

  this.subject().reloadRelationship(mockModel, 'summary', true);
  assert.equal(
    pretender.handledRequests[0].url,
    '/v1/job/job-1/summary?index=1',
    'First request is a blocking request for job-1 summary relationship'
  );

  return wait().then(() => {
    this.subject().reloadRelationship(mockModel, 'summary', true);
    assert.equal(
      pretender.handledRequests[1].url,
      '/v1/job/job-1/summary?index=2',
      'Second request is a blocking request with an incremented index param'
    );
  });
});

test('findAll can be canceled', function(assert) {
  const { pretender } = this.server;
  pretender.get('/v1/jobs', () => [200, {}, '[]'], true);

  this.subject()
    .findAll(null, { modelName: 'job' }, null, {
      reload: true,
      adapterOptions: { watch: true },
    })
    .catch(() => {});

  const { request: xhr } = pretender.requestReferences[0];
  assert.equal(xhr.status, 0, 'Request is still pending');

  // Schedule the cancelation before waiting
  run.next(() => {
    this.subject().cancelFindAll('job');
  });

  return wait().then(() => {
    assert.ok(xhr.aborted, 'Request was aborted');
  });
});

test('findRecord can be canceled', function(assert) {
  const { pretender } = this.server;
  const jobId = JSON.stringify(['job-1', 'default']);

  pretender.get('/v1/job/:id', () => [200, {}, '{}'], true);

  this.subject().findRecord(null, { modelName: 'job' }, jobId, {
    reload: true,
    adapterOptions: { watch: true },
  });

  const { request: xhr } = pretender.requestReferences[0];
  assert.equal(xhr.status, 0, 'Request is still pending');

  // Schedule the cancelation before waiting
  run.next(() => {
    this.subject().cancelFindRecord('job', jobId);
  });

  return wait().then(() => {
    assert.ok(xhr.aborted, 'Request was aborted');
  });
});

test('relationship reloads can be canceled', function(assert) {
  const { pretender } = this.server;
  const plainId = 'job-1';
  const mockModel = makeMockModel(plainId);
  pretender.get('/v1/job/:id/summary', () => [200, {}, '{}'], true);

  this.subject().reloadRelationship(mockModel, 'summary', true);

  const { request: xhr } = pretender.requestReferences[0];
  assert.equal(xhr.status, 0, 'Request is still pending');

  // Schedule the cancelation before waiting
  run.next(() => {
    this.subject().cancelReloadRelationship(mockModel, 'summary');
  });

  return wait().then(() => {
    assert.ok(xhr.aborted, 'Request was aborted');
  });
});

test('requests can be canceled even if multiple requests for the same URL were made', function(assert) {
  const { pretender } = this.server;
  const jobId = JSON.stringify(['job-1', 'default']);

  pretender.get('/v1/job/:id', () => [200, {}, '{}'], true);

  this.subject().findRecord(null, { modelName: 'job' }, jobId, {
    reload: true,
    adapterOptions: { watch: true },
  });

  this.subject().findRecord(null, { modelName: 'job' }, jobId, {
    reload: true,
    adapterOptions: { watch: true },
  });

  const { request: xhr } = pretender.requestReferences[0];
  assert.equal(xhr.status, 0, 'Request is still pending');
  assert.equal(pretender.requestReferences.length, 2, 'Two findRecord requests were made');
  assert.equal(
    pretender.requestReferences.mapBy('url').uniq().length,
    1,
    'The two requests have the same URL'
  );

  // Schedule the cancelation before waiting
  run.next(() => {
    this.subject().cancelFindRecord('job', jobId);
  });

  return wait().then(() => {
    assert.ok(xhr.aborted, 'Request was aborted');
  });
});

function makeMockModel(id, options) {
  return assign(
    {
      relationshipFor(name) {
        return {
          kind: 'belongsTo',
          type: 'job-summary',
          key: name,
        };
      },
      belongsTo(name) {
        return {
          link() {
            return `/v1/job/${id}/${name}`;
          },
        };
      },
    },
    options
  );
}
