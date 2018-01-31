import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import { click, find } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import moment from 'moment';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

moduleForComponent(
  'job-page/parts/running-deployment',
  'Integration | Component | job-page/parts/running-deployment',
  {
    integration: true,
    beforeEach() {
      fragmentSerializerInitializer(getOwner(this));
      window.localStorage.clear();
      this.store = getOwner(this).lookup('service:store');
      this.server = startMirage();
      this.server.create('namespace');
    },
    afterEach() {
      this.server.shutdown();
      window.localStorage.clear();
    },
  }
);

test('there is no active deployment section when the job has no active deployment', function(assert) {
  this.server.create('job', {
    type: 'service',
    noActiveDeployment: true,
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    this.set('job', this.store.peekAll('job').get('firstObject'));
    this.render(hbs`
      {{job-page/parts/running-deployment job=job}})
    `);

    return wait().then(() => {
      assert.notOk(find('[data-test-active-deployment]'), 'No active deployment');
    });
  });
});

test('the active deployment section shows up for the currently running deployment', function(assert) {
  this.server.create('job', { type: 'service', createAllocations: false, activeDeployment: true });

  this.store.findAll('job');

  return wait().then(() => {
    this.set('job', this.store.peekAll('job').get('firstObject'));
    this.render(hbs`
      {{job-page/parts/running-deployment job=job}}
    `);

    return wait().then(() => {
      const deployment = this.get('job.runningDeployment');
      const version = deployment.get('version');

      assert.ok(find('[data-test-active-deployment]'), 'Active deployment');
      assert.equal(
        find('[data-test-active-deployment-stat="id"]').textContent.trim(),
        deployment.get('shortId'),
        'The active deployment is the most recent running deployment'
      );

      assert.equal(
        find('[data-test-active-deployment-stat="submit-time"]').textContent.trim(),
        moment(version.get('submitTime')).fromNow(),
        'Time since the job was submitted is in the active deployment header'
      );

      assert.equal(
        find('[data-test-deployment-metric="canaries"]').textContent.trim(),
        `${deployment.get('placedCanaries')} / ${deployment.get('desiredCanaries')}`,
        'Canaries, both places and desired, are in the metrics'
      );

      assert.equal(
        find('[data-test-deployment-metric="placed"]').textContent.trim(),
        deployment.get('placedAllocs'),
        'Placed allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="desired"]').textContent.trim(),
        deployment.get('desiredTotal'),
        'Desired allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="healthy"]').textContent.trim(),
        deployment.get('healthyAllocs'),
        'Healthy allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="unhealthy"]').textContent.trim(),
        deployment.get('unhealthyAllocs'),
        'Unhealthy allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-notification]').textContent.trim(),
        deployment.get('statusDescription'),
        'Status description is in the metrics block'
      );
    });
  });
});

test('the active deployment section can be expanded to show task groups and allocations', function(assert) {
  this.server.create('node');
  this.server.create('job', { type: 'service', activeDeployment: true });

  this.store.findAll('job');

  return wait().then(() => {
    this.set('job', this.store.peekAll('job').get('firstObject'));
    this.render(hbs`
      {{job-page/parts/running-deployment job=job}}
    `);

    return wait().then(() => {
      assert.notOk(find('[data-test-deployment-task-groups]'), 'Task groups not found');
      assert.notOk(find('[data-test-deployment-allocations]'), 'Allocations not found');

      click('[data-test-deployment-toggle-details]');

      return wait().then(() => {
        assert.ok(find('[data-test-deployment-task-groups]'), 'Task groups found');
        assert.ok(find('[data-test-deployment-allocations]'), 'Allocations found');
      });
    });
  });
});
