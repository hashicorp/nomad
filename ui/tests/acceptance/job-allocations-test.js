/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL, click } from '@ember/test-helpers';
import { getPageTitle } from 'ember-page-title/test-support';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Allocations from 'nomad-ui/tests/pages/jobs/job/allocations';

let job;
let allocations;

const makeSearchAllocations = (server) => {
  Array(10)
    .fill(null)
    .map((_, index) => {
      server.create('allocation', {
        id: index < 5 ? `ffffff-dddddd-${index}` : `111111-222222-${index}`,
        shallow: true,
      });
    });
};

module('Acceptance | job allocations', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    this.server.create('node-pool');
    this.server.create('node');

    job = this.server.create('job', {
      noFailedPlacements: true,
      createAllocations: false,
    });
  });

  test('it passes an accessibility audit', async function (assert) {
    this.server.createList('allocation', Allocations.pageSize - 1, {
      shallow: true,
    });
    allocations = this.server.schema.allocations.where({
      jobId: job.id,
    }).models;

    await Allocations.visit({ id: job.id });
    await a11yAudit(assert);
  });

  test('lists all allocations for the job', async function (assert) {
    this.server.createList('allocation', Allocations.pageSize - 1, {
      shallow: true,
    });
    allocations = this.server.schema.allocations.where({
      jobId: job.id,
    }).models;

    await Allocations.visit({ id: job.id });

    assert.deepEqual(
      Allocations.allocations.length,
      Allocations.pageSize - 1,
      'Allocations are shown in a table',
    );

    const sortedAllocations = allocations.sortBy('modifyIndex').reverse();

    Allocations.allocations.forEach((allocation, index) => {
      const shortId = sortedAllocations[index].id.split('-')[0];
      assert.deepEqual(
        allocation.shortId,
        shortId,
        `Allocation ${index} is ${shortId}`,
      );
    });

    assert.deepEqual(getPageTitle(), `Job ${job.name} allocations - Nomad`);
  });

  test('clicking an allocation results in the correct endpoint being hit', async function (assert) {
    this.server.createList('allocation', Allocations.pageSize - 1, {
      shallow: true,
    });
    allocations = this.server.schema.allocations.where({
      jobId: job.id,
    }).models;

    await Allocations.visit({ id: job.id });

    const firstAllocation = document.querySelector('[data-test-allocation]');
    await click(firstAllocation);
    const requestToAllocationEndpoint =
      this.server.pretender.handledRequests.find((request) =>
        request.url.includes(
          `/v1/allocation/${firstAllocation.dataset.testAllocation}`,
        ),
      );

    assert.ok(requestToAllocationEndpoint, 'the correct endpoint is hit');

    assert.deepEqual(
      currentURL(),
      `/allocations/${firstAllocation.dataset.testAllocation}`,
      'the URL is correct',
    );
  });

  test('allocations table is sortable', async function (assert) {
    this.server.createList('allocation', Allocations.pageSize - 1);
    allocations = this.server.schema.allocations.where({
      jobId: job.id,
    }).models;

    await Allocations.visit({ id: job.id });
    await Allocations.sortBy('taskGroupName');

    assert.deepEqual(
      currentURL(),
      `/jobs/${job.id}/allocations?sort=taskGroupName`,
      'the URL persists the sort parameter',
    );
    const sortedAllocations = allocations.sortBy('taskGroup').reverse();
    Allocations.allocations.forEach((allocation, index) => {
      const shortId = sortedAllocations[index].id.split('-')[0];
      assert.deepEqual(
        allocation.shortId,
        shortId,
        `Allocation ${index} is ${shortId} with task group ${sortedAllocations[index].taskGroup}`,
      );
    });
  });

  test('allocations table is searchable', async function (assert) {
    makeSearchAllocations(this.server);

    allocations = this.server.schema.allocations.where({
      jobId: job.id,
    }).models;

    await Allocations.visit({ id: job.id });
    await Allocations.search('ffffff');

    assert.deepEqual(
      Allocations.allocations.length,
      5,
      'List is filtered by search term',
    );
  });

  test('when a search yields no results, the search box remains', async function (assert) {
    makeSearchAllocations(this.server);

    allocations = this.server.schema.allocations.where({
      jobId: job.id,
    }).models;

    await Allocations.visit({ id: job.id });
    await Allocations.search('^nothing will ever match this long regex$');

    assert.deepEqual(
      Allocations.emptyState.headline,
      'No Matches',
      'List is empty and the empty state is about search',
    );

    assert.ok(Allocations.hasSearchBox, 'Search box is still shown');
  });

  test('when the job for the allocations is not found, an error message is shown, but the URL persists', async function (assert) {
    await Allocations.visit({ id: 'not-a-real-job' });

    assert.deepEqual(
      this.server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made',
    );
    assert.deepEqual(
      currentURL(),
      '/jobs/not-a-real-job/allocations',
      'The URL persists',
    );
    assert.ok(Allocations.error.isPresent, 'Error message is shown');
    assert.deepEqual(
      Allocations.error.title,
      'Not Found',
      'Error message is for 404',
    );
  });

  testFacet('Status', {
    facet: Allocations.facets.status,
    paramName: 'status',
    expectedOptions: [
      'Pending',
      'Running',
      'Complete',
      'Failed',
      'Lost',
      'Unknown',
    ],
    async beforeEach() {
      ['pending', 'running', 'complete', 'failed', 'lost', 'unknown'].forEach(
        (s) => {
          this.server.createList('allocation', 5, { clientStatus: s });
        },
      );
      await Allocations.visit({ id: job.id });
    },
    filter: (alloc, selection) =>
      alloc.jobId == job.id && selection.includes(alloc.clientStatus),
  });

  testFacet('Client', {
    facet: Allocations.facets.client,
    paramName: 'client',
    expectedOptions(allocs) {
      return Array.from(
        new Set(
          allocs
            .filter((alloc) => alloc.jobId == job.id)
            .mapBy('nodeId')
            .map((id) => id.split('-')[0]),
        ),
      ).sort();
    },
    async beforeEach() {
      this.server.createList('node', 5);
      this.server.createList('allocation', 20);

      await Allocations.visit({ id: job.id });
    },
    filter: (alloc, selection) =>
      alloc.jobId == job.id && selection.includes(alloc.nodeId.split('-')[0]),
  });

  testFacet('Task Group', {
    facet: Allocations.facets.taskGroup,
    paramName: 'taskGroup',
    expectedOptions(allocs) {
      return Array.from(
        new Set(
          allocs.filter((alloc) => alloc.jobId == job.id).mapBy('taskGroup'),
        ),
      ).sort();
    },
    async beforeEach() {
      this.server.create('node-pool');
      job = this.server.create('job', {
        type: 'service',
        status: 'running',
        groupsCount: 5,
      });

      await Allocations.visit({ id: job.id });
    },
    filter: (alloc, selection) =>
      alloc.jobId == job.id && selection.includes(alloc.taskGroup),
  });
});

function testFacet(
  label,
  { facet, paramName, beforeEach, filter, expectedOptions },
) {
  test(`facet ${label} | the ${label} facet has the correct options`, async function (assert) {
    await beforeEach.call(this);
    await facet.toggle();

    let expectation;
    if (typeof expectedOptions === 'function') {
      expectation = expectedOptions.call(this, this.server.db.allocations);
    } else {
      expectation = expectedOptions;
    }

    assert.deepEqual(
      facet.options.map((option) => option.label.trim()),
      expectation,
      'Options for facet are as expected',
    );
  });

  test(`facet ${label} | the ${label} facet filters the allocations list by ${label}`, async function (assert) {
    let option;

    await beforeEach.call(this);

    await facet.toggle();
    option = facet.options.objectAt(0);
    await option.toggle();

    const selection = [option.key];
    const expectedAllocs = this.server.db.allocations
      .filter((alloc) => filter(alloc, selection))
      .sortBy('modifyIndex')
      .reverse();

    Allocations.allocations.forEach((alloc, index) => {
      assert.deepEqual(
        alloc.id,
        expectedAllocs[index].id,
        `Allocation at ${index} is ${expectedAllocs[index].id}`,
      );
    });
  });

  test(`facet ${label} | selecting multiple options in the ${label} facet results in a broader search`, async function (assert) {
    const selection = [];

    await beforeEach.call(this);
    await facet.toggle();

    const option1 = facet.options.objectAt(0);
    const option2 = facet.options.objectAt(1);
    await option1.toggle();
    selection.push(option1.key);
    await option2.toggle();
    selection.push(option2.key);

    const expectedAllocs = this.server.db.allocations
      .filter((alloc) => filter(alloc, selection))
      .sortBy('modifyIndex')
      .reverse();

    Allocations.allocations.forEach((alloc, index) => {
      assert.deepEqual(
        alloc.id,
        expectedAllocs[index].id,
        `Allocation at ${index} is ${expectedAllocs[index].id}`,
      );
    });
  });

  test(`facet ${label} | selecting options in the ${label} facet updates the ${paramName} query param`, async function (assert) {
    const selection = [];

    await beforeEach.call(this);
    await facet.toggle();

    const option1 = facet.options.objectAt(0);
    const option2 = facet.options.objectAt(1);
    await option1.toggle();
    selection.push(option1.key);
    await option2.toggle();
    selection.push(option2.key);

    assert.deepEqual(
      currentURL(),
      `/jobs/${job.id}/allocations?${paramName}=${encodeURIComponent(
        JSON.stringify(selection),
      )}`,
      'URL has the correct query param key and value',
    );
  });
}
