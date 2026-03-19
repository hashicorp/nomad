/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { TrackedObject } from 'tracked-built-ins';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import JobStatusFailedOrLost from 'nomad-ui/components/job-status/failed-or-lost';

module('Integration | Component | job-status/failed-or-lost', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    const state = new TrackedObject({
      job: {
        id: 'job1',
      },
      allocs: [
        {
          id: 1,
          name: 'alloc1',
        },
        {
          id: 2,
          name: 'alloc2',
        },
      ],
    });

    await render(
      <template>
        <JobStatusFailedOrLost
          @job={{state.job}}
          @restartedAllocs={{state.allocs}}
        />
      </template>,
    );

    assert.dom('h4').hasText('Replaced Allocations');
    assert.dom('.failed-or-lost-links').hasText('2 Restarted');
    await componentA11yAudit(this.element, assert);
  });

  test('it links or does not link appropriately', async function (assert) {
    const state = new TrackedObject({
      job: {
        id: 'job1',
      },
      allocs: [
        {
          id: 1,
          name: 'alloc1',
        },
        {
          id: 2,
          name: 'alloc2',
        },
      ],
    });

    await render(
      <template>
        <JobStatusFailedOrLost
          @restartedAllocs={{state.allocs}}
          @job={{state.job}}
        />
      </template>,
    );

    assert.dom('.failed-or-lost-links > span > *:last-child').hasTagName('a');
    state.allocs = [];
    assert
      .dom('.failed-or-lost-links > span > *:last-child')
      .doesNotHaveTagName('a');
  });

  test('it shows rescheduling as well', async function (assert) {
    const state = new TrackedObject({
      job: {
        id: 'job1',
      },
      restartedAllocs: [
        {
          id: 1,
          name: 'alloc1',
        },
        {
          id: 2,
          name: 'alloc2',
        },
      ],
      rescheduledAllocs: [
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
      ],
      supportsRescheduling: true,
    });

    await render(
      <template>
        <JobStatusFailedOrLost
          @restartedAllocs={{state.restartedAllocs}}
          @rescheduledAllocs={{state.rescheduledAllocs}}
          @job={{state.job}}
          @supportsRescheduling={{state.supportsRescheduling}}
        />
      </template>,
    );

    assert.dom('.failed-or-lost-links').containsText('2 Restarted');
    assert.dom('.failed-or-lost-links').containsText('3 Rescheduled');
    state.supportsRescheduling = false;
    assert.dom('.failed-or-lost-links').doesNotContainText('Rescheduled');
  });
});
