import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import { click, find, findAll } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

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
    this.setProperties({
      job,
      sortProperty: 'name',
      sortDescending: true,
      currentPage: 1,
      gotoJob: () => {},
    });

    this.render(hbs`
      {{job-page/periodic
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        currentPage=currentPage
        gotoJob=gotoJob}}
    `);

    return wait().then(() => {
      const currentJobCount = server.db.jobs.length;

      assert.equal(
        findAll('[data-test-job-name]').length,
        childrenCount,
        'The new periodic job launch is in the children list'
      );

      click('[data-test-force-launch]');

      return wait().then(() => {
        const id = job.get('plainId');
        const namespace = job.get('namespace.name') || 'default';
        let expectedURL = `/v1/job/${id}/periodic/force`;
        if (namespace !== 'default') {
          expectedURL += `?namespace=${namespace}`;
        }

        assert.ok(
          server.pretender.handledRequests
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
  server.pretender.post('/v1/job/:id/periodic/force', () => [403, {}, null]);

  this.server.create('job', 'periodic', {
    id: 'parent',
    childrenCount: 1,
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    const job = this.store.peekAll('job').findBy('plainId', 'parent');
    this.setProperties({
      job,
      sortProperty: 'name',
      sortDescending: true,
      currentPage: 1,
      gotoJob: () => {},
    });

    this.render(hbs`
      {{job-page/periodic
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        currentPage=currentPage
        gotoJob=gotoJob}}
    `);

    return wait().then(() => {
      assert.notOk(find('[data-test-force-error-title]'), 'No error message yet');

      click('[data-test-force-launch]');

      return wait().then(() => {
        assert.equal(
          find('[data-test-force-error-title]').textContent,
          'Could Not Force Launch',
          'Appropriate error is shown'
        );
        assert.ok(
          find('[data-test-force-error-body]').textContent.includes('ACL'),
          'The error message mentions ACLs'
        );

        click('[data-test-force-error-close]');

        assert.notOk(find('[data-test-force-error-title]'), 'Error message is dismissable');
      });
    });
  });
});
