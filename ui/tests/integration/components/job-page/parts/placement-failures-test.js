/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
/* Mirage fixtures are random so we can't expect a set number of assertions */
import hbs from 'htmlbars-inline-precompile';
import { findAll, find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module(
  'Integration | Component | job-page/parts/placement-failures',
  function (hooks) {
    setupRenderingTest(hooks);

    hooks.beforeEach(function () {
      fragmentSerializerInitializer(this.owner);
      window.localStorage.clear();
      this.store = this.owner.lookup('service:store');
      this.server = startMirage();
      this.server.create('namespace');
      this.server.create('node-pool');
    });

    hooks.afterEach(function () {
      this.server.shutdown();
      window.localStorage.clear();
    });

    test('when the job has placement failures, they are called out', async function (assert) {
      this.server.create('job', {
        failedPlacements: true,
        createAllocations: false,
      });
      await this.store.findAll('job');

      const job = this.store.peekAll('job').get('firstObject');
      await job.reload();

      this.set('job', job);

      await render(hbs`
      <JobPage::Parts::PlacementFailures @job={{job}} />)
    `);

      const failedEvaluation = this.get('job.evaluations')
        .filterBy('hasPlacementFailures')
        .sortBy('modifyIndex')
        .reverse()
        .get('firstObject');
      const failedTGAllocs = failedEvaluation.get('failedTGAllocs');

      assert.ok(
        find('[data-test-placement-failures]'),
        'Placement failures section found'
      );

      const taskGroupLabels = findAll(
        '[data-test-placement-failure-task-group]'
      ).map((title) => title.textContent.trim());

      failedTGAllocs.forEach((alloc) => {
        const name = alloc.get('name');
        assert.ok(
          taskGroupLabels.find((label) => label.includes(name)),
          `${name} included in placement failures list`
        );
        assert.ok(
          taskGroupLabels.find((label) =>
            label.includes(alloc.get('coalescedFailures') + 1)
          ),
          'The number of unplaced allocs = CoalescedFailures + 1'
        );
      });

      await componentA11yAudit(this.element, assert);
    });

    test('when the job has no placement failures, the placement failures section is gone', async function (assert) {
      this.server.create('job', {
        noFailedPlacements: true,
        createAllocations: false,
      });
      await this.store.findAll('job');

      const job = this.store.peekAll('job').get('firstObject');
      await job.reload();

      this.set('job', job);

      await render(hbs`
      <JobPage::Parts::PlacementFailures @job={{job}} />)
    `);

      assert.notOk(
        find('[data-test-placement-failures]'),
        'Placement failures section not found'
      );
    });
  }
);
