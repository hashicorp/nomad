/**
 * Copyright IBM Corp. 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | group selections', function (hooks) {
  setupRenderingTest(hooks);

  test('unplaced choices are shown separately from allocation counts', async function (assert) {
    this.set('job', {
      effectiveGroupSelectionStatuses: {
        runtime: { Count: 2, Unplaced: 1 },
      },
    });
    await render(hbs`<JobStatus::GroupSelections @job={{this.job}} />`);
    assert
      .dom('[data-test-group-selection="runtime"]')
      .includesText('1/2 task groups selected');
    assert
      .dom('[data-test-group-selection="runtime"]')
      .includesText('1 unplaced');
  });

  test('satisfied choices have no unplaced warning', async function (assert) {
    this.set('job', {
      effectiveGroupSelectionStatuses: {
        runtime: { Count: 1, Unplaced: 0 },
      },
    });
    await render(hbs`<JobStatus::GroupSelections @job={{this.job}} />`);
    assert
      .dom('[data-test-group-selection="runtime"]')
      .hasText('runtime: 1/1 task groups selected');
  });
});
