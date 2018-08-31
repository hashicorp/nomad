import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import { test, moduleForComponent } from 'ember-qunit';
import { click, find } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { startJob, stopJob, expectError, expectDeleteRequest, expectStartRequest } from './helpers';
import Job from 'nomad-ui/tests/pages/jobs/detail';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

moduleForComponent('job-page/service', 'Integration | Component | job-page/service', {
  integration: true,
  beforeEach() {
    Job.setContext(this);
    fragmentSerializerInitializer(getOwner(this));
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
    .then(expectError(assert, 'Could Not Stop Job'));
});

test('Starting a job sends a post request for the job using the current definition', function(assert) {
  let job;

  const mirageJob = makeMirageJob(this.server, { status: 'dead' });
  this.store.findAll('job');

  return wait()
    .then(() => {
      job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(startJob)
    .then(() => expectStartRequest(assert, this.server, job));
});

test('Starting a job without proper permissions shows an error message', function(assert) {
  this.server.pretender.post('/v1/job/:id', () => [403, {}, null]);

  const mirageJob = makeMirageJob(this.server, { status: 'dead' });
  this.store.findAll('job');

  return wait()
    .then(() => {
      const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(startJob)
    .then(expectError(assert, 'Could Not Start Job'));
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

test('Active deployment can be promoted', function(assert) {
  let job;
  let deployment;

  this.server.create('node');
  const mirageJob = makeMirageJob(this.server, { activeDeployment: true });

  this.store.findAll('job');

  return wait()
    .then(() => {
      job = this.store.peekAll('job').findBy('plainId', mirageJob.id);
      deployment = job.get('latestDeployment');

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(() => {
      click('[data-test-promote-canary]');
      return wait();
    })
    .then(() => {
      const requests = this.server.pretender.handledRequests;
      assert.ok(
        requests
          .filterBy('method', 'POST')
          .findBy('url', `/v1/deployment/promote/${deployment.get('id')}`),
        'A promote POST request was made'
      );
    });
});

test('When promoting the active deployment fails, an error is shown', function(assert) {
  this.server.pretender.post('/v1/deployment/promote/:id', () => [403, {}, null]);

  let job;

  this.server.create('node');
  const mirageJob = makeMirageJob(this.server, { activeDeployment: true });

  this.store.findAll('job');

  return wait()
    .then(() => {
      job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

      this.setProperties(commonProperties(job));
      this.render(commonTemplate);

      return wait();
    })
    .then(() => {
      click('[data-test-promote-canary]');
      return wait();
    })
    .then(() => {
      assert.equal(
        find('[data-test-job-error-title]').textContent,
        'Could Not Promote Deployment',
        'Appropriate error is shown'
      );
      assert.ok(
        find('[data-test-job-error-body]').textContent.includes('ACL'),
        'The error message mentions ACLs'
      );

      click('[data-test-job-error-close]');
      assert.notOk(find('[data-test-job-error-title]'), 'Error message is dismissable');
      return wait();
    });
});
