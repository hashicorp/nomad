import { run } from '@ember/runloop';
import { assign } from '@ember/polyfills';
import { settled } from '@ember/test-helpers';
import { setupTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import XHRToken from 'nomad-ui/utils/classes/xhr-token';

module('Unit | Adapter | Job', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(async function() {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('job');

    window.sessionStorage.clear();
    window.localStorage.clear();

    this.server = startMirage();

    this.initializeUI = async () => {
      this.server.create('namespace');
      this.server.create('namespace', { id: 'some-namespace' });
      this.server.create('node');
      this.server.create('job', { id: 'job-1', namespaceId: 'default' });
      this.server.create('job', { id: 'job-2', namespaceId: 'some-namespace' });

      this.server.create('region', { id: 'region-1' });
      this.server.create('region', { id: 'region-2' });

      this.system = this.owner.lookup('service:system');

      // Namespace, default region, and all regions are requests that all
      // job requests depend on. Fetching them ahead of time means testing
      // job adapter behavior in isolation.
      await this.system.get('namespaces');
      this.system.get('shouldIncludeRegion');
      await this.system.get('defaultRegion');

      // Reset the handledRequests array to avoid accounting for this
      // namespaces request everywhere.
      this.server.pretender.handledRequests.length = 0;
    };
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  test('The job endpoint is the only required endpoint for fetching a job', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const jobName = 'job-1';
    const jobNamespace = 'default';
    const jobId = JSON.stringify([jobName, jobNamespace]);

    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`],
      'The only request made is /job/:id'
    );
  });

  test('When a namespace is set in localStorage but a job in the default namespace is requested, the namespace query param is not present', async function(assert) {
    window.localStorage.nomadActiveNamespace = 'some-namespace';

    await this.initializeUI();

    const { pretender } = this.server;
    const jobName = 'job-1';
    const jobNamespace = 'default';
    const jobId = JSON.stringify([jobName, jobNamespace]);

    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`],
      'The only request made is /job/:id with no namespace query param'
    );
  });

  test('When a namespace is in localStorage and the requested job is in the default namespace, the namespace query param is left out', async function(assert) {
    window.localStorage.nomadActiveNamespace = 'red-herring';

    await this.initializeUI();

    const { pretender } = this.server;
    const jobName = 'job-1';
    const jobNamespace = 'default';
    const jobId = JSON.stringify([jobName, jobNamespace]);

    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`],
      'The request made is /job/:id with no namespace query param'
    );
  });

  test('When the job has a namespace other than default, it is in the URL', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const jobName = 'job-2';
    const jobNamespace = 'some-namespace';
    const jobId = JSON.stringify([jobName, jobNamespace]);

    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}?namespace=${jobNamespace}`],
      'The only request made is /job/:id?namespace=:namespace'
    );
  });

  test('When there is no token set in the token service, no x-nomad-token header is set', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const jobId = JSON.stringify(['job-1', 'default']);

    this.subject().findRecord(null, { modelName: 'job' }, jobId);

    assert.notOk(
      pretender.handledRequests.mapBy('requestHeaders').some(headers => headers['X-Nomad-Token']),
      'No token header present on either job request'
    );
  });

  test('When a token is set in the token service, then x-nomad-token header is set', async function(assert) {
    await this.initializeUI();

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

  test('findAll can be watched', async function(assert) {
    await this.initializeUI();

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

    await settled();
    request();
    assert.equal(
      pretender.handledRequests[1].url,
      '/v1/jobs?index=2',
      'Third request is a blocking request with an incremented index param'
    );

    await settled();
  });

  test('findRecord can be watched', async function(assert) {
    await this.initializeUI();

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

    await settled();
    request();
    assert.equal(
      pretender.handledRequests[1].url,
      '/v1/job/job-1?index=2',
      'Third request is a blocking request with an incremented index param'
    );

    await settled();
  });

  test('relationships can be reloaded', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const plainId = 'job-1';
    const mockModel = makeMockModel(plainId);

    this.subject().reloadRelationship(mockModel, 'summary');
    await settled();
    assert.equal(
      pretender.handledRequests[0].url,
      `/v1/job/${plainId}/summary`,
      'Relationship was reloaded'
    );
  });

  test('relationship reloads can be watched', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const plainId = 'job-1';
    const mockModel = makeMockModel(plainId);

    this.subject().reloadRelationship(mockModel, 'summary', { watch: true });
    assert.equal(
      pretender.handledRequests[0].url,
      '/v1/job/job-1/summary?index=1',
      'First request is a blocking request for job-1 summary relationship'
    );

    await settled();
    this.subject().reloadRelationship(mockModel, 'summary', { watch: true });
    assert.equal(
      pretender.handledRequests[1].url,
      '/v1/job/job-1/summary?index=2',
      'Second request is a blocking request with an incremented index param'
    );
  });

  test('findAll can be canceled', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const token = new XHRToken();

    pretender.get('/v1/jobs', () => [200, {}, '[]'], true);

    this.subject()
      .findAll(null, { modelName: 'job' }, null, {
        reload: true,
        adapterOptions: { watch: true, abortToken: token },
      })
      .catch(() => {});

    const { request: xhr } = pretender.requestReferences[0];
    assert.equal(xhr.status, 0, 'Request is still pending');

    // Schedule the cancelation before waiting
    run.next(() => {
      token.abort();
    });

    await settled();
    assert.ok(xhr.aborted, 'Request was aborted');
  });

  test('findRecord can be canceled', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const jobId = JSON.stringify(['job-1', 'default']);
    const token = new XHRToken();

    pretender.get('/v1/job/:id', () => [200, {}, '{}'], true);

    this.subject().findRecord(null, { modelName: 'job' }, jobId, {
      reload: true,
      adapterOptions: { watch: true, abortToken: token },
    });

    const { request: xhr } = pretender.requestReferences[0];
    assert.equal(xhr.status, 0, 'Request is still pending');

    // Schedule the cancelation before waiting
    run.next(() => {
      token.abort();
    });

    await settled();
    assert.ok(xhr.aborted, 'Request was aborted');
  });

  test('relationship reloads can be canceled', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const plainId = 'job-1';
    const token = new XHRToken();
    const mockModel = makeMockModel(plainId);
    pretender.get('/v1/job/:id/summary', () => [200, {}, '{}'], true);

    this.subject().reloadRelationship(mockModel, 'summary', { watch: true, abortToken: token });

    const { request: xhr } = pretender.requestReferences[0];
    assert.equal(xhr.status, 0, 'Request is still pending');

    // Schedule the cancelation before waiting
    run.next(() => {
      token.abort();
    });

    await settled();
    assert.ok(xhr.aborted, 'Request was aborted');
  });

  test('requests can be canceled even if multiple requests for the same URL were made', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const jobId = JSON.stringify(['job-1', 'default']);
    const token1 = new XHRToken();
    const token2 = new XHRToken();

    pretender.get('/v1/job/:id', () => [200, {}, '{}'], true);

    this.subject().findRecord(null, { modelName: 'job' }, jobId, {
      reload: true,
      adapterOptions: { watch: true, abortToken: token1 },
    });

    this.subject().findRecord(null, { modelName: 'job' }, jobId, {
      reload: true,
      adapterOptions: { watch: true, abortToken: token2 },
    });

    const { request: xhr } = pretender.requestReferences[0];
    const { request: xhr2 } = pretender.requestReferences[1];
    assert.equal(xhr.status, 0, 'Request is still pending');
    assert.equal(pretender.requestReferences.length, 2, 'Two findRecord requests were made');
    assert.equal(
      pretender.requestReferences.mapBy('url').uniq().length,
      1,
      'The two requests have the same URL'
    );

    // Schedule the cancelation and resolution before waiting
    run.next(() => {
      token1.abort();
      pretender.resolve(xhr2);
    });

    await settled();
    assert.ok(xhr.aborted, 'Request one was aborted');
    assert.notOk(xhr2.aborted, 'Request two was not aborted');
  });

  test('when there is no region set, requests are made without the region query param', async function(assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const jobName = 'job-1';
    const jobNamespace = 'default';
    const jobId = JSON.stringify([jobName, jobNamespace]);

    await settled();
    this.subject().findRecord(null, { modelName: 'job' }, jobId);
    this.subject().findAll(null, { modelName: 'job' }, null);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}`, '/v1/jobs'],
      'No requests include the region query param'
    );
  });

  test('when there is a region set, requests are made with the region query param', async function(assert) {
    const region = 'region-2';
    window.localStorage.nomadActiveRegion = region;

    await this.initializeUI();

    const { pretender } = this.server;
    const jobName = 'job-1';
    const jobNamespace = 'default';
    const jobId = JSON.stringify([jobName, jobNamespace]);

    await settled();
    this.subject().findRecord(null, { modelName: 'job' }, jobId);
    this.subject().findAll(null, { modelName: 'job' }, null);

    assert.deepEqual(
      pretender.handledRequests.mapBy('url'),
      [`/v1/job/${jobName}?region=${region}`, `/v1/jobs?region=${region}`],
      'Requests include the region query param'
    );
  });

  test('when the region is set to the default region, requests are made without the region query param', async function(assert) {
    window.localStorage.nomadActiveRegion = 'region-1';

    await this.initializeUI();

    const { pretender } = this.server;
    const jobName = 'job-1';
    const jobNamespace = 'default';
    const jobId = JSON.stringify([jobName, jobNamespace]);

    await settled();
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
