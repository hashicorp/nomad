import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { stopJob, expectStopError, expectDeleteRequest } from './helpers';

moduleForComponent('job-page/service', 'Integration | Component | job-page/service', {
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

const makeMirageJob = server =>
  server.create('job', {
    type: 'service',
    createAllocations: false,
    status: 'running',
  });

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
