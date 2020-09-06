import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Allocations from 'nomad-ui/tests/pages/jobs/job/allocations';

let job;
let allocations;

const makeSearchAllocations = server => {
  Array(10)
    .fill(null)
    .map((_, index) => {
      server.create('allocation', {
        id: index < 5 ? `ffffff-dddddd-${index}` : `111111-222222-${index}`,
        shallow: true,
      });
    });
};

module('Acceptance | job allocations', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('node');

    job = server.create('job', { noFailedPlacements: true, createAllocations: false });
  });

  test('it passes an accessibility audit', async function(assert) {
    server.createList('allocation', Allocations.pageSize - 1, { shallow: true });
    allocations = server.schema.allocations.where({ jobId: job.id }).models;

    await Allocations.visit({ id: job.id });
    await a11yAudit(assert);
  });

  test('lists all allocations for the job', async function(assert) {
    server.createList('allocation', Allocations.pageSize - 1, { shallow: true });
    allocations = server.schema.allocations.where({ jobId: job.id }).models;

    await Allocations.visit({ id: job.id });

    assert.equal(
      Allocations.allocations.length,
      Allocations.pageSize - 1,
      'Allocations are shown in a table'
    );

    const sortedAllocations = allocations.sortBy('modifyIndex').reverse();

    Allocations.allocations.forEach((allocation, index) => {
      const shortId = sortedAllocations[index].id.split('-')[0];
      assert.equal(allocation.shortId, shortId, `Allocation ${index} is ${shortId}`);
    });

    assert.equal(document.title, `Job ${job.name} allocations - Nomad`);
  });

  test('allocations table is sortable', async function(assert) {
    server.createList('allocation', Allocations.pageSize - 1);
    allocations = server.schema.allocations.where({ jobId: job.id }).models;

    await Allocations.visit({ id: job.id });
    await Allocations.sortBy('taskGroupName');

    assert.equal(
      currentURL(),
      `/jobs/${job.id}/allocations?sort=taskGroupName`,
      'the URL persists the sort parameter'
    );
    const sortedAllocations = allocations.sortBy('taskGroup').reverse();
    Allocations.allocations.forEach((allocation, index) => {
      const shortId = sortedAllocations[index].id.split('-')[0];
      assert.equal(
        allocation.shortId,
        shortId,
        `Allocation ${index} is ${shortId} with task group ${sortedAllocations[index].taskGroup}`
      );
    });
  });

  test('allocations table is searchable', async function(assert) {
    makeSearchAllocations(server);

    allocations = server.schema.allocations.where({ jobId: job.id }).models;

    await Allocations.visit({ id: job.id });
    await Allocations.search('ffffff');

    assert.equal(Allocations.allocations.length, 5, 'List is filtered by search term');
  });

  test('when a search yields no results, the search box remains', async function(assert) {
    makeSearchAllocations(server);

    allocations = server.schema.allocations.where({ jobId: job.id }).models;

    await Allocations.visit({ id: job.id });
    await Allocations.search('^nothing will ever match this long regex$');

    assert.equal(
      Allocations.emptyState.headline,
      'No Matches',
      'List is empty and the empty state is about search'
    );

    assert.ok(Allocations.hasSearchBox, 'Search box is still shown');
  });

  test('when the job for the allocations is not found, an error message is shown, but the URL persists', async function(assert) {
    await Allocations.visit({ id: 'not-a-real-job' });

    assert.equal(
      server.pretender.handledRequests
        .filter(request => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/allocations', 'The URL persists');
    assert.ok(Allocations.error.isPresent, 'Error message is shown');
    assert.equal(Allocations.error.title, 'Not Found', 'Error message is for 404');
  });
});
