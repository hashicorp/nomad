import { run } from '@ember/runloop';
import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import { findAll } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

moduleForComponent(
  'job-page/parts/evaluations',
  'Integration | Component | job-page/parts/evaluations',
  {
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
  }
);

test('lists all evaluations for the job', function(assert) {
  let job;

  this.server.create('job', { noFailedPlacements: true, createAllocations: false });
  this.store.findAll('job');

  return wait().then(() => {
    run(() => {
      job = this.store.peekAll('job').get('firstObject');
    });

    this.setProperties({ job });

    this.render(hbs`
      {{job-page/parts/evaluations job=job}}
    `);

    return wait().then(() => {
      const evaluationRows = findAll('[data-test-evaluation]');
      assert.equal(
        evaluationRows.length,
        job.get('evaluations.length'),
        'All evaluations are listed'
      );

      job
        .get('evaluations')
        .sortBy('modifyIndex')
        .reverse()
        .forEach((evaluation, index) => {
          assert.equal(
            evaluationRows[index].querySelector('[data-test-id]').textContent.trim(),
            evaluation.get('shortId'),
            `Evaluation ${index} is ${evaluation.get('shortId')}`
          );
        });
    });
  });
});
