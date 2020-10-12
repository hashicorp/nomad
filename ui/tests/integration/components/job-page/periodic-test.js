import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, find, findAll, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import moment from 'moment';
import { create, collection } from 'ember-cli-page-object';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import pageSizeSelect from 'nomad-ui/tests/acceptance/behaviors/page-size-select';
import pageSizeSelectPageObject from 'nomad-ui/tests/pages/components/page-size-select';
import {
  jobURL,
  stopJob,
  startJob,
  expectError,
  expectDeleteRequest,
  expectStartRequest,
} from './helpers';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

// A minimum viable page object to use with the pageSizeSelect behavior
const PeriodicJobPage = create({
  pageSize: 25,
  jobs: collection('[data-test-job-row]'),
  pageSizeSelect: pageSizeSelectPageObject(),
});

module('Integration | Component | job-page/periodic', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    window.localStorage.clear();
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
  });

  hooks.afterEach(function() {
    this.server.shutdown();
    window.localStorage.clear();
  });

  const commonTemplate = hbs`
    <JobPage::Periodic
      @job={{job}}
      @sortProperty={{sortProperty}}
      @sortDescending={{sortDescending}}
      @currentPage={{currentPage}}
      @gotoJob={{gotoJob}} />
  `;

  const commonProperties = job => ({
    job,
    sortProperty: 'name',
    sortDescending: true,
    currentPage: 1,
    gotoJob: () => {},
  });

  test('Clicking Force Launch launches a new periodic child job', async function(assert) {
    const childrenCount = 3;

    this.server.create('job', 'periodic', {
      id: 'parent',
      childrenCount,
      createAllocations: false,
    });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(commonProperties(job));
    await this.render(commonTemplate);

    const currentJobCount = server.db.jobs.length;

    assert.equal(
      findAll('[data-test-job-name]').length,
      childrenCount,
      'The new periodic job launch is in the children list'
    );

    await click('[data-test-force-launch]');

    const expectedURL = jobURL(job, '/periodic/force');

    assert.ok(
      this.server.pretender.handledRequests
        .filterBy('method', 'POST')
        .find(req => req.url === expectedURL),
      'POST URL was correct'
    );

    assert.equal(server.db.jobs.length, currentJobCount + 1, 'POST request was made');
  });

  test('Clicking force launch without proper permissions shows an error message', async function(assert) {
    this.server.pretender.post('/v1/job/:id/periodic/force', () => [403, {}, '']);

    this.server.create('job', 'periodic', {
      id: 'parent',
      childrenCount: 1,
      createAllocations: false,
      status: 'running',
    });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(commonProperties(job));
    await this.render(commonTemplate);

    assert.notOk(find('[data-test-job-error-title]'), 'No error message yet');

    await click('[data-test-force-launch]');

    assert.equal(
      find('[data-test-job-error-title]').textContent,
      'Could Not Force Launch',
      'Appropriate error is shown'
    );
    assert.ok(
      find('[data-test-job-error-body]').textContent.includes('ACL'),
      'The error message mentions ACLs'
    );

    await click('[data-test-job-error-close]');

    assert.notOk(find('[data-test-job-error-title]'), 'Error message is dismissable');
  });

  test('Stopping a job sends a delete request for the job', async function(assert) {
    const mirageJob = this.server.create('job', 'periodic', {
      childrenCount: 0,
      createAllocations: false,
      status: 'running',
    });

    let job;
    await this.store.findAll('job');

    job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);
    await stopJob();

    expectDeleteRequest(assert, this.server, job);
  });

  test('Stopping a job without proper permissions shows an error message', async function(assert) {
    this.server.pretender.delete('/v1/job/:id', () => [403, {}, '']);

    const mirageJob = this.server.create('job', 'periodic', {
      childrenCount: 0,
      createAllocations: false,
      status: 'running',
    });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await stopJob();
    expectError(assert, 'Could Not Stop Job');

    await componentA11yAudit(this.element, assert);
  });

  test('Starting a job sends a post request for the job using the current definition', async function(assert) {
    const mirageJob = this.server.create('job', 'periodic', {
      childrenCount: 0,
      createAllocations: false,
      status: 'dead',
    });
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await startJob();
    expectStartRequest(assert, this.server, job);
  });

  test('Starting a job without proper permissions shows an error message', async function(assert) {
    this.server.pretender.post('/v1/job/:id', () => [403, {}, '']);

    const mirageJob = this.server.create('job', 'periodic', {
      childrenCount: 0,
      createAllocations: false,
      status: 'dead',
    });
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await startJob();
    expectError(assert, 'Could Not Start Job');
  });

  test('Each job row includes the submitted time', async function(assert) {
    this.server.create('job', 'periodic', {
      id: 'parent',
      childrenCount: 1,
      createAllocations: false,
    });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(commonProperties(job));
    await this.render(commonTemplate);

    assert.equal(
      find('[data-test-job-submit-time]').textContent,
      moment(job.get('children.firstObject.submitTime')).format('MMM DD HH:mm:ss ZZ'),
      'The new periodic job launch is in the children list'
    );
  });

  pageSizeSelect({
    resourceName: 'job',
    pageObject: PeriodicJobPage,
    pageObjectList: PeriodicJobPage.jobs,
    async setup() {
      this.server.create('job', 'periodic', {
        id: 'parent',
        childrenCount: PeriodicJobPage.pageSize,
        createAllocations: false,
      });

      await this.store.findAll('job');

      const job = this.store.peekAll('job').findBy('plainId', 'parent');

      this.setProperties(commonProperties(job));
      await this.render(commonTemplate);
    },
  });
});
