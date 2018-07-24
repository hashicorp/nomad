import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { stopJob, expectStopError, expectDeleteRequest } from './helpers';
import Job from 'nomad-ui/tests/pages/jobs/detail';

moduleForComponent('job-page/service', 'Integration | Component | job-page/service', {
  integration: true,
  beforeEach() {
    Job.setContext(this);
    window.localStorage.clear();
    this.store = getOwner(this).lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
  },
  afterEach() {
    Job.removeContext();
    this.server.shutdown();
    window.localStorage.clear();
  },
});

const commonTemplate = hbs`
  {{job-page/service
    job=job
    sortProperty=sortProperty
    sortDescending=sortDescending
    currentPage=currentPage
    gotoJob=gotoJob}}
`;

const commonProperties = job => ({
  job,
  sortProperty: 'name',
  sortDescending: true,
  currentPage: 1,
  gotoJob() {},
});

const makeMirageJob = (server, props = {}) =>
  server.create(
    'job',
    assign(
      {
        type: 'service',
        createAllocations: false,
        status: 'running',
      },
      props
    )
  );

test('Stopping a job sends a delete request for the job', function(assert) {
  let job;

  const mirageJob = makeMirageJob(this.server);
  this.store.findAll('job');

  return wait()
    .then(() => {
      job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(stopJob)
    .then(() => expectDeleteRequest(assert, this.server, job));
});

test('Stopping a job without proper permissions shows an error message', function(assert) {
  this.server.pretender.delete('/v1/job/:id', () => [403, {}, null]);

  const mirageJob = makeMirageJob(this.server);
  this.store.findAll('job');

  return wait()
    .then(() => {
      const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(stopJob)
    .then(expectStopError(assert));
});

test('Recent allocations shows allocations in the job context', function(assert) {
  let job;

  this.server.create('node');
  const mirageJob = makeMirageJob(this.server, { createAllocations: true });
  this.store.findAll('job');

  return wait()
    .then(() => {
      job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(() => {
      const allocation = this.server.db.allocations.sortBy('modifyIndex').reverse()[0];
      const allocationRow = Job.allocations.objectAt(0);

      assert.equal(allocationRow.shortId, allocation.id.split('-')[0], 'ID');
      assert.equal(allocationRow.taskGroup, allocation.taskGroup, 'Task Group name');
    });
});

test('Recent allocations caps out at five', function(assert) {
  let job;

  this.server.create('node');
  const mirageJob = makeMirageJob(this.server);
  this.server.createList('allocation', 10);

  this.store.findAll('job');

  return wait().then(() => {
    job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    this.render(commonTemplate);

    return wait().then(() => {
      assert.equal(Job.allocations.length, 5, 'Capped at 5 allocations');
      assert.ok(
        Job.viewAllAllocations.includes(job.get('allocations.length') + ''),
        `View link mentions ${job.get('allocations.length')} allocations`
      );
    });
  });
});

test('Recent allocations shows an empty message when the job has no allocations', function(assert) {
  let job;

  this.server.create('node');
  const mirageJob = makeMirageJob(this.server);

  this.store.findAll('job');

  return wait()
    .then(() => {
      job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(() => {
      assert.ok(
        Job.recentAllocationsEmptyState.headline.includes('No Allocations'),
        'No allocations empty message'
      );
    });
});
