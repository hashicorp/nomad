/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL } from '@ember/test-helpers';
import { getPageTitle } from 'ember-page-title/test-support';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Clients from 'nomad-ui/tests/pages/jobs/job/clients';
import setPolicy from 'nomad-ui/tests/utils/set-policy';

let job;
let clients;

const makeSearchableClients = (server, job) => {
  Array(10)
    .fill(null)
    .map((_, index) => {
      const node = server.create('node', {
        id: index < 5 ? `ffffff-dddddd-${index}` : `111111-222222-${index}`,
        datacenter: 'dc1',
        status: 'ready',
      });
      server.create('allocation', { jobId: job.id, nodeId: node.id });
    });
};

module('Acceptance | job clients', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    setPolicy.call(this, {
      id: 'node-read',
      name: 'node-read',
      rulesJSON: {
        Node: {
          Policy: 'read',
        },
      },
    });

    this.server.createList('node-pool', 5);
    clients = this.server.createList('node', 12, {
      datacenter: 'dc1',
      status: 'ready',
    });
    // Job with 1 task group.
    job = this.server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'sysbatch',
      resourceSpec: ['M: 256, C: 500'],
      createAllocations: false,
    });
    clients.forEach((c) => {
      this.server.create('allocation', { jobId: job.id, nodeId: c.id });
    });

    // Create clients without allocations to have some 'not scheduled' job status.
    clients = clients.concat(
      this.server.createList('node', 3, {
        datacenter: 'dc1',
        status: 'ready',
      }),
    );
  });

  test('it passes an accessibility audit', async function (assert) {
    await Clients.visit({ id: job.id });
    await a11yAudit(assert);
  });

  test('lists all clients for the job', async function (assert) {
    await Clients.visit({ id: job.id });
    assert.deepEqual(
      Clients.clients.length,
      15,
      'Clients are shown in a table',
    );

    const clientIDs = clients.sortBy('id').map((c) => c.id);
    const clientsInTable = Clients.clients.map((c) => c.id).sort();
    assert.deepEqual(clientsInTable, clientIDs);

    assert.deepEqual(getPageTitle(), `Job ${job.name} clients - Nomad`);
  });

  test('dates have tooltip', async function (assert) {
    await Clients.visit({ id: job.id });

    Clients.clients.forEach((clientRow, index) => {
      const jobStatus = Clients.clientFor(clientRow.id).status;

      ['createTime', 'modifyTime'].forEach((col) => {
        if (jobStatus === 'not scheduled') {
          assert.deepEqual(
            clientRow[col].text,
            '-',
            `row ${index} doesn't have ${col} tooltip`,
          );
          return;
        }

        const hasTooltip = clientRow[col].tooltip.isPresent;
        const tooltipText = clientRow[col].tooltip.text;
        assert.true(hasTooltip, `row ${index} has ${col} tooltip`);
        assert.ok(
          tooltipText,
          `row ${index} has ${col} tooltip content ${tooltipText}`,
        );
      });
    });
  });

  test('clients table is sortable', async function (assert) {
    await Clients.visit({ id: job.id });
    await Clients.sortBy('node.name');

    assert.deepEqual(
      currentURL(),
      `/jobs/${job.id}/clients?desc=true&sort=node.name`,
      'the URL persists the sort parameter',
    );

    const sortedClients = clients.sortBy('name').reverse();
    Clients.clients.forEach((client, index) => {
      const shortId = sortedClients[index].id.split('-')[0];
      assert.deepEqual(
        client.shortId,
        shortId,
        `Client ${index} is ${shortId} with name ${sortedClients[index].name}`,
      );
    });
  });

  test('clients table is searchable', async function (assert) {
    makeSearchableClients(this.server, job);

    await Clients.visit({ id: job.id });
    await Clients.search('ffffff');

    assert.deepEqual(
      Clients.clients.length,
      5,
      'List is filtered by search term',
    );
  });

  test('when a search yields no results, the search box remains', async function (assert) {
    makeSearchableClients(this.server, job);

    await Clients.visit({ id: job.id });
    await Clients.search('^nothing will ever match this long regex$');

    assert.deepEqual(
      Clients.emptyState.headline,
      'No Matches',
      'List is empty and the empty state is about search',
    );

    assert.ok(Clients.hasSearchBox, 'Search box is still shown');
  });

  test('when the job for the clients is not found, an error message is shown, but the URL persists', async function (assert) {
    await Clients.visit({ id: 'not-a-real-job' });

    assert.deepEqual(
      this.server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made',
    );
    assert.deepEqual(
      currentURL(),
      '/jobs/not-a-real-job/clients',
      'The URL persists',
    );
    assert.ok(Clients.error.isPresent, 'Error message is shown');
    assert.deepEqual(
      Clients.error.title,
      'Not Found',
      'Error message is for 404',
    );
  });

  test('clicking row goes to client details', async function (assert) {
    const client = clients[0];

    await Clients.visit({ id: job.id });
    await Clients.clientFor(client.id).click();
    assert.deepEqual(currentURL(), `/clients/${client.id}`);

    await Clients.visit({ id: job.id });
    await Clients.clientFor(client.id).visit();
    assert.deepEqual(currentURL(), `/clients/${client.id}`);

    await Clients.visit({ id: job.id });
    await Clients.clientFor(client.id).visitRow();
    assert.deepEqual(currentURL(), `/clients/${client.id}`);
  });

  testFacet('Job Status', {
    facet: Clients.facets.jobStatus,
    paramName: 'jobStatus',
    expectedOptions: [
      'Queued',
      'Not Scheduled',
      'Starting',
      'Running',
      'Complete',
      'Degraded',
      'Failed',
      'Lost',
      'Unknown',
    ],
    async beforeEach() {
      await Clients.visit({ id: job.id });
    },
  });

  function testFacet(label, { facet, paramName, beforeEach, expectedOptions }) {
    test(`the ${label} facet has the correct options`, async function (assert) {
      await beforeEach.call(this);
      await facet.toggle();

      let expectation;
      if (typeof expectedOptions === 'function') {
        expectation = expectedOptions.call(this);
      } else {
        expectation = expectedOptions;
      }

      assert.deepEqual(
        facet.options.map((option) => option.label.trim()),
        expectation,
        `Options for facet ${paramName} are as expected`,
      );
    });

    // TODO: add facet tests for actual list filtering
  }
});
