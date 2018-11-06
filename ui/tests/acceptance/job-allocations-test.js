import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import Allocations from 'nomad-ui/tests/pages/jobs/job/allocations';

let job;
let allocations;

const makeSearchAllocations = server => {
  Array(10)
    .fill(null)
    .map((_, index) => {
      server.create('allocation', {
        id: index < 5 ? `ffffff-dddddd-${index}` : `111111-222222-${index}`,
      });
    });
};

moduleForAcceptance('Acceptance | job allocations', {
  beforeEach() {
    server.create('node');

    job = server.create('job', { noFailedPlacements: true, createAllocations: false });
  },
});

test('lists all allocations for the job', function(assert) {
  server.createList('allocation', Allocations.pageSize - 1);
  allocations = server.schema.allocations.where({ jobId: job.id }).models;

  Allocations.visit({ id: job.id });

  andThen(() => {
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
  });
});

test('allocations table is sortable', function(assert) {
  server.createList('allocation', Allocations.pageSize - 1);
  allocations = server.schema.allocations.where({ jobId: job.id }).models;

  Allocations.visit({ id: job.id });

  andThen(() => {
    Allocations.sortBy('taskGroupName');

    andThen(() => {
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
  });
});

test('allocations table is searchable', function(assert) {
  makeSearchAllocations(server);

  allocations = server.schema.allocations.where({ jobId: job.id }).models;
  Allocations.visit({ id: job.id });

  andThen(() => {
    Allocations.search('ffffff');
  });

  andThen(() => {
    assert.equal(Allocations.allocations.length, 5, 'List is filtered by search term');
  });
});

test('when a search yields no results, the search box remains', function(assert) {
  makeSearchAllocations(server);

  allocations = server.schema.allocations.where({ jobId: job.id }).models;
  Allocations.visit({ id: job.id });

  andThen(() => {
    Allocations.search('^nothing will ever match this long regex$');
  });

  andThen(() => {
    assert.equal(
      Allocations.emptyState.headline,
      'No Matches',
      'List is empty and the empty state is about search'
    );

    assert.ok(Allocations.hasSearchBox, 'Search box is still shown');
  });
});

test('when the job for the allocations is not found, an error message is shown, but the URL persists', function(assert) {
  Allocations.visit({ id: 'not-a-real-job' });

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/allocations', 'The URL persists');
    assert.ok(Allocations.error.isPresent, 'Error message is shown');
    assert.equal(Allocations.error.title, 'Not Found', 'Error message is for 404');
  });
});
