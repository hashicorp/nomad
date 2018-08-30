import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import { click, find, findAll } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import {
  jobURL,
  stopJob,
  startJob,
  expectError,
  expectDeleteRequest,
  expectStartRequest,
} from './helpers';

moduleForComponent('job-page/periodic', 'Integration | Component | job-page/periodic', {
  integration: true,
  beforeEach() {
    window.localStorage.clear();
    this.store = getOwner(this).lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
  },
  afterEach() {
    this.server.shutdown();
    window.localStorage.clear();
  },
});

const commonTemplate = hbs`
  {{job-page/periodic
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
  gotoJob: () => {},
});

test('Clicking Force Launch launches a new periodic child job', function(assert) {
  const childrenCount = 3;

  this.server.create('job', 'periodic', {
    id: 'parent',
    childrenCount,
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    const job = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(commonProperties(job));
    this.render(commonTemplate);

    return wait().then(() => {
      const currentJobCount = server.db.jobs.length;

      assert.equal(
        findAll('[data-test-job-name]').length,
        childrenCount,
        'The new periodic job launch is in the children list'
      );

      click('[data-test-force-launch]');

      return wait().then(() => {
        const expectedURL = jobURL(job, '/periodic/force');

        assert.ok(
          this.server.pretender.handledRequests
            .filterBy('method', 'POST')
            .find(req => req.url === expectedURL),
          'POST URL was correct'
        );

        assert.equal(server.db.jobs.length, currentJobCount + 1, 'POST request was made');
      });
    });
  });
});

test('Clicking force launch without proper permissions shows an error message', function(assert) {
  this.server.pretender.post('/v1/job/:id/periodic/force', () => [403, {}, null]);

  this.server.create('job', 'periodic', {
    id: 'parent',
    childrenCount: 1,
    createAllocations: false,
    status: 'running',
  });

  this.store.findAll('job');

  return wait().then(() => {
    const job = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(commonProperties(job));
    this.render(commonTemplate);

    return wait().then(() => {
      assert.notOk(find('[data-test-job-error-title]'), 'No error message yet');

      click('[data-test-force-launch]');

      return wait().then(() => {
        assert.equal(
          find('[data-test-job-error-title]').textContent,
          'Could Not Force Launch',
          'Appropriate error is shown'
        );
        assert.ok(
          find('[data-test-job-error-body]').textContent.includes('ACL'),
          'The error message mentions ACLs'
        );

        click('[data-test-job-error-close]');

        assert.notOk(find('[data-test-job-error-title]'), 'Error message is dismissable');
      });
    });
  });
});

test('Stopping a job sends a delete request for the job', function(assert) {
  const mirageJob = this.server.create('job', 'periodic', {
    childrenCount: 0,
    createAllocations: false,
    status: 'running',
  });

  let job;
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

  const mirageJob = this.server.create('job', 'periodic', {
    childrenCount: 0,
    createAllocations: false,
    status: 'running',
  });

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

  const mirageJob = this.server.create('job', 'periodic', {
    childrenCount: 0,
    createAllocations: false,
    status: 'dead',
  });
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

  const mirageJob = this.server.create('job', 'periodic', {
    childrenCount: 0,
    createAllocations: false,
    status: 'dead',
  });
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
