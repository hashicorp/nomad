import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Clients from 'nomad-ui/tests/pages/jobs/job/clients';

let job;
let clients;

const makeSearchableClients = server => {
  Array(10)
    .fill(null)
    .map((_, index) => {
      server.create('node', {
        id: index < 5 ? `ffffff-dddddd-${index}` : `111111-222222-${index}`,
        shallow: true,
      });
    });
};

module('Acceptance | job clients', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    clients = server.createList('node', 12, {
      datacenter: 'dc1',
      status: 'ready',
    });
    // Job with 1 task group.
    job = server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'sysbatch',
      resourceSpec: ['M: 256, C: 500'],
      createAllocations: false,
    });
    clients.forEach(c => {
      server.create('allocation', { jobId: job.id, nodeId: c.id });
    });
  });

  test('it passes an accessibility audit', async function(assert) {
    await Clients.visit({ id: job.id });
    await a11yAudit(assert);
  });

  test('lists all clients for the job', async function(assert) {
    await Clients.visit({ id: job.id });
    assert.equal(Clients.clients.length, 12, 'Clients are shown in a table');

    const sortedClients = clients;

    Clients.clients.forEach((client, index) => {
      const shortId = sortedClients[index].id.split('-')[0];
      console.log('client\n\n', client);
      assert.equal(client.shortId, shortId, `Client ${index} is ${shortId}`);
    });

    assert.equal(document.title, `Job ${job.name} clients - Nomad`);
  });

  test('clients table is sortable', async function(assert) {
    await Clients.visit({ id: job.id });
    await Clients.sortBy('modifyTime');

    assert.equal(
      currentURL(),
      `/jobs/${job.id}/clients?desc=true&sort=modifyTime`,
      'the URL persists the sort parameter'
    );
    const sortedClients = clients.sortBy('modifyTime').reverse();
    Clients.clients.forEach((client, index) => {
      const shortId = sortedClients[index].id.split('-')[0];
      assert.equal(
        client.shortId,
        shortId,
        `Client ${index} is ${shortId} with modify time ${sortedClients[index].modifyTime}`
      );
    });
  });

  test('clients table is searchable', async function(assert) {
    makeSearchableClients(server);

    clients = server.schema.nodes.where({ jobId: job.id }).models;

    await Clients.visit({ id: job.id });
    await Clients.search('ffffff');

    assert.equal(Clients.clients.length, 5, 'List is filtered by search term');
  });

  test('when a search yields no results, the search box remains', async function(assert) {
    makeSearchableClients(server);

    clients = server.schema.nodes.where({ jobId: job.id }).models;

    await Clients.visit({ id: job.id });
    await Clients.search('^nothing will ever match this long regex$');

    assert.equal(
      Clients.emptyState.headline,
      'No Matches',
      'List is empty and the empty state is about search'
    );

    assert.ok(Clients.hasSearchBox, 'Search box is still shown');
  });

  test('when the job for the clients is not found, an error message is shown, but the URL persists', async function(assert) {
    await Clients.visit({ id: 'not-a-real-job' });

    assert.equal(
      server.pretender.handledRequests
        .filter(request => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/clients', 'The URL persists');
    assert.ok(Clients.error.isPresent, 'Error message is shown');
    assert.equal(Clients.error.title, 'Not Found', 'Error message is for 404');
  });
});
