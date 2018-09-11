import EmberObject from '@ember/object';
import { getOwner } from '@ember/application';
import { run } from '@ember/runloop';
import { assign } from '@ember/polyfills';
import { test } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import moduleForAdapter from '../../helpers/module-for-adapter';

moduleForAdapter('job', 'Unit | Adapter | Job', {
  needs: [
    'adapter:application',
    'adapter:job',
    'adapter:namespace',
    'model:task-group',
    'model:allocation',
    'model:deployment',
    'model:evaluation',
    'model:job-summary',
    'model:job-version',
    'model:namespace',
    'model:task-group-summary',
    'serializer:namespace',
    'serializer:job',
    'serializer:job-summary',
    'service:token',
    'service:system',
    'service:watchList',
    'transform:fragment',
    'transform:fragment-array',
  ],
  beforeEach() {
    window.sessionStorage.clear();
    window.localStorage.clear();

    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('namespace', { id: 'some-namespace' });
    this.server.create('node');
    this.server.create('job', { id: 'job-1', namespaceId: 'default' });
    this.server.create('job', { id: 'job-2', namespaceId: 'some-namespace' });

    this.server.create('region', { id: 'region-1' });
    this.server.create('region', { id: 'region-2' });

    this.system = getOwner(this).lookup('service:system');

    // Namespace, default region, and all regions are requests that all
    // job requests depend on. Fetching them ahead of time means testing
    // job adapter behavior in isolation.
    this.system.get('namespaces');
    this.system.get('shouldIncludeRegion');
    this.system.get('defaultRegion');

    // Reset the handledRequests array to avoid accounting for this
    // namespaces request everywhere.
    this.server.pretender.handledRequests.length = 0;
  },
  afterEach() {
    this.server.shutdown();
  },
});

test('The job endpoint is the only required endpoint for fetching a job', function(assert) {
  const { pretender } = this.server;
  const jobName = 'job-1';
  const jobNamespace = 'default';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`],
      'The only request made is /job/:id'
    );
  });
});

test('When a namespace is set in localStorage but a job in the default namespace is requested, the namespace query param is not present', function(assert) {
  window.localStorage.nomadActiveNamespace = 'some-namespace';

  const { pretender } = this.server;
  const jobName = 'job-1';
  const jobNamespace = 'default';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  this.system.get('namespaces');
  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`],
      'The only request made is /job/:id with no namespace query param'
    );
  });
});

test('When a namespace is in localStorage and the requested job is in the default namespace, the namespace query param is left out', function(assert) {
  window.localStorage.nomadActiveNamespace = 'red-herring';

  const { pretender } = this.server;
  const jobName = 'job-1';
  const jobNamespace = 'default';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`],
      'The request made is /job/:id with no namespace query param'
    );
  });
});

test('When the job has a namespace other than default, it is in the URL', function(assert) {
  const { pretender } = this.server;
  const jobName = 'job-2';
  const jobNamespace = 'some-namespace';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}?namespace=${jobNamespace}`],
      'The only request made is /job/:id?namespace=:namespace'
    );
  });
});

test('When there is no token set in the token service, no x-nomad-token header is set', function(assert) {
  const { pretender } = this.server;
  const jobId = JSON.stringify(['job-1', 'default']);

  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.notOk(
      pretender.handledRequests.mapBy('requestHeaders').some(headers => headers['X-Nomad-Token']),
      'No token header present on either job request'
    );
  });
});

test('When a token is set in the token service, then x-nomad-token header is set', function(assert) {
  const { pretender } = this.server;
  const jobId = JSON.stringify(['job-1', 'default']);
  const secret = 'here is the secret';

  return wait().then(() => {
    this.subject().set('token.secret', secret);
    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.ok(
      pretender.handledRequests
        .mapBy('requestHeaders')
        .every(headers => headers['X-Nomad-Token'] === secret),
      'The token header is present on both job requests'
    );
  });
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
    '/v1/jobs?index=1',
    'Second request is a blocking request for jobs'
  );

  return wait().then(() => {
    request();
    assert.equal(
      pretender.handledRequests[1].url,
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
    '/v1/job/job-1?index=1',
    'Second request is a blocking request for job-1'
  );

  return wait().then(() => {
    request();
    assert.equal(
      pretender.handledRequests[1].url,
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
  return wait().then(() => {
    assert.equal(
      pretender.handledRequests[0].url,
      `/v1/job/${plainId}/summary`,
      'Relationship was reloaded'
    );
  });
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

test('canceling a find record request will never cancel a request with the same url but different method', function(assert) {
  const { pretender } = this.server;
  const jobId = JSON.stringify(['job-1', 'default']);

  pretender.get('/v1/job/:id', () => [200, {}, '{}'], true);
  pretender.delete('/v1/job/:id', () => [204, {}, ''], 200);

  this.subject().findRecord(null, { modelName: 'job' }, jobId, {
    reload: true,
    adapterOptions: { watch: true },
  });

  this.subject().stop(EmberObject.create({ id: jobId }));

  const { request: getXHR } = pretender.requestReferences[0];
  const { request: deleteXHR } = pretender.requestReferences[1];
  assert.equal(getXHR.status, 0, 'Get request is still pending');
  assert.equal(deleteXHR.status, 0, 'Delete request is still pending');

  // Schedule the cancelation before waiting
  run.next(() => {
    this.subject().cancelFindRecord('job', jobId);
  });

  return wait().then(() => {
    assert.ok(getXHR.aborted, 'Get request was aborted');
    assert.notOk(deleteXHR.aborted, 'Delete request was aborted');
  });
});

test('when there is no region set, requests are made without the region query param', function(assert) {
  const { pretender } = this.server;
  const jobName = 'job-1';
  const jobNamespace = 'default';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);
    this.subject().findAll(null, { modelName: 'job' }, null);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`, '/v1/jobs'],
      'No requests include the region query param'
    );
  });
});

test('when there is a region set, requests are made with the region query param', function(assert) {
  const region = 'region-2';
  window.localStorage.nomadActiveRegion = region;

  const { pretender } = this.server;
  const jobName = 'job-1';
  const jobNamespace = 'default';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);
    this.subject().findAll(null, { modelName: 'job' }, null);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}?region=${region}`, `/v1/jobs?region=${region}`],
      'Requests include the region query param'
    );
  });
});

test('when the region is set to the default region, requests are made without the region query param', function(assert) {
  window.localStorage.nomadActiveRegion = 'region-1';

  const { pretender } = this.server;
  const jobName = 'job-1';
  const jobNamespace = 'default';
  const jobId = JSON.stringify([jobName, jobNamespace]);

  return wait().then(() => {
    this.subject().findRecord(null, { modelName: 'job' }, jobId);
    this.subject().findAll(null, { modelName: 'job' }, null);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`, '/v1/jobs'],
      'No requests include the region query param'
    );
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
