/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | job-status/failed-or-lost', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    assert.expect(3);

    let job = {
      id: 'job1',
    };

    let allocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    this.set('allocs', allocs);
    this.set('job', job);

    await render(hbs`<JobStatus::FailedOrLost
      @job={{this.job}}
      @restartedAllocs={{this.allocs}}
    />`);

    assert.dom('h4').hasText('Replaced Allocations');
    assert.dom('.failed-or-lost-links').hasText('2 Restarted');
    await componentA11yAudit(this.element, assert);
  });

  test('it links or does not link appropriately', async function (assert) {
    let job = {
      id: 'job1',
    };

    let allocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    this.set('allocs', allocs);
    this.set('job', job);

    await render(hbs`<JobStatus::FailedOrLost
      @restartedAllocs={{this.allocs}}
      @job={{this.job}}
    />`);

    // Ensure it's of type a
    assert.dom('.failed-or-lost-links > span > *:last-child').hasTagName('a');
    this.set('allocs', []);
    assert
      .dom('.failed-or-lost-links > span > *:last-child')
      .doesNotHaveTagName('a');
  });

  test('it shows rescheduling as well', async function (assert) {
    let job = {
      id: 'job1',
    };

    let restartedAllocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    let rescheduledAllocs = [
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

    this.set('restartedAllocs', restartedAllocs);
    this.set('rescheduledAllocs', rescheduledAllocs);
    this.set('job', job);
    this.set('supportsRescheduling', true);

    await render(hbs`<JobStatus::FailedOrLost
      @restartedAllocs={{this.restartedAllocs}}
      @rescheduledAllocs={{this.rescheduledAllocs}}
      @job={{this.job}}
      @supportsRescheduling={{this.supportsRescheduling}}
    />`);

    assert.dom('.failed-or-lost-links').containsText('2 Restarted');
    assert.dom('.failed-or-lost-links').containsText('3 Rescheduled');
    this.set('supportsRescheduling', false);
    assert.dom('.failed-or-lost-links').doesNotContainText('Rescheduled');
  });
});
