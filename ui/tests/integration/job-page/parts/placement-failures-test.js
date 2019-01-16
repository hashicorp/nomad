import { getOwner } from '@ember/application';
import { run } from '@ember/runloop';
import hbs from 'htmlbars-inline-precompile';
import wait from 'ember-test-helpers/wait';
import { findAll, find } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

moduleForComponent(
  'job-page/parts/placement-failures',
  'Integration | Component | job-page/parts/placement-failures',
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

test('when the job has placement failures, they are called out', function(assert) {
  this.server.create('job', { failedPlacements: true, createAllocations: false });
  this.store.findAll('job').then(jobs => {
    jobs.forEach(job => job.reload());
  });

  return wait().then(() => {
    run(() => {
      this.set('job', this.store.peekAll('job').get('firstObject'));
    });

    this.render(hbs`
      {{job-page/parts/placement-failures job=job}})
    `);

    return wait().then(() => {
      const failedEvaluation = this.get('job.evaluations')
        .filterBy('hasPlacementFailures')
        .sortBy('modifyIndex')
        .reverse()
        .get('firstObject');
      const failedTGAllocs = failedEvaluation.get('failedTGAllocs');

      assert.ok(find('[data-test-placement-failures]'), 'Placement failures section found');

      const taskGroupLabels = findAll('[data-test-placement-failure-task-group]').map(title =>
        title.textContent.trim()
      );

      failedTGAllocs.forEach(alloc => {
        const name = alloc.get('name');
        assert.ok(
          taskGroupLabels.find(label => label.includes(name)),
          `${name} included in placement failures list`
        );
        assert.ok(
          taskGroupLabels.find(label => label.includes(alloc.get('coalescedFailures') + 1)),
          'The number of unplaced allocs = CoalescedFailures + 1'
        );
      });
    });
  });
});

test('when the job has no placement failures, the placement failures section is gone', function(assert) {
  this.server.create('job', { noFailedPlacements: true, createAllocations: false });
  this.store.findAll('job');

  return wait().then(() => {
    run(() => {
      this.set('job', this.store.peekAll('job').get('firstObject'));
    });

    this.render(hbs`
      {{job-page/parts/placement-failures job=job}})
    `);

    return wait().then(() => {
      assert.notOk(find('[data-test-placement-failures]'), 'Placement failures section not found');
    });
  });
});
