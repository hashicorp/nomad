/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import { tracked } from '@glimmer/tracking';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import JobStatusFailedOrLost from 'nomad-ui/components/job-status/failed-or-lost';

class FailedOrLostTestState {
  @tracked allocs;
  @tracked restartedAllocs;
  @tracked rescheduledAllocs;
  @tracked supportsRescheduling;

  constructor() {
    this.allocs = [];
    this.restartedAllocs = [];
    this.rescheduledAllocs = [];
    this.supportsRescheduling = false;
  }
}

module('Integration | Component | job-status/failed-or-lost', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    const job = { id: 'job1' };
    const state = new FailedOrLostTestState();
    state.allocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    await render(
      <template>
        <JobStatusFailedOrLost @job={{job}} @restartedAllocs={{state.allocs}} />
      </template>,
    );

    assert.dom('h4').hasText('Replaced Allocations');
    assert.dom('.failed-or-lost-links').hasText('2 Restarted');
    await componentA11yAudit(this.element, assert);
  });

  test('it links or does not link appropriately', async function (assert) {
    const job = { id: 'job1' };
    const state = new FailedOrLostTestState();
    state.allocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    await render(
      <template>
        <JobStatusFailedOrLost @restartedAllocs={{state.allocs}} @job={{job}} />
      </template>,
    );

    assert.dom('.failed-or-lost-links > span > *:last-child').hasTagName('a');
    state.allocs = [];
    await settled();
    assert
      .dom('.failed-or-lost-links > span > *:last-child')
      .doesNotHaveTagName('a');
  });

  test('it shows rescheduling as well', async function (assert) {
    const job = { id: 'job1' };
    const state = new FailedOrLostTestState();
    state.restartedAllocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    state.rescheduledAllocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
      {
        id: 3,
        name: 'alloc3',
      },
    ];
    state.supportsRescheduling = true;

    await render(
      <template>
        <JobStatusFailedOrLost
          @restartedAllocs={{state.restartedAllocs}}
          @rescheduledAllocs={{state.rescheduledAllocs}}
          @job={{job}}
          @supportsRescheduling={{state.supportsRescheduling}}
        />
      </template>,
    );

    assert.dom('.failed-or-lost-links').containsText('2 Restarted');
    assert.dom('.failed-or-lost-links').containsText('3 Rescheduled');
    state.supportsRescheduling = false;
    await settled();
    assert.dom('.failed-or-lost-links').doesNotContainText('Rescheduled');
  });
});
