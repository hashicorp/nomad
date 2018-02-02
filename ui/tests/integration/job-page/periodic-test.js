import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import { click, findAll } from 'ember-native-dom-helpers';
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

        assert.ok(
          server.pretender.handledRequests
            .filterBy('method', 'POST')
            .find(req => req.url === `/v1/job/${id}/periodic/force?namespace=${namespace}`),
          'POST URL was correct'
        );

        assert.ok(server.db.jobs.length, currentJobCount + 1, 'POST request was made');

        return wait().then(() => {
          assert.equal(
            findAll('[data-test-job-name]').length,
            childrenCount + 1,
            'The new periodic job launch is in the children list'
          );
        });
      });
    });
  });
});
