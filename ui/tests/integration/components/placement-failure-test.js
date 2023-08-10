/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, findAll, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { assign } from '@ember/polyfills';
import hbs from 'htmlbars-inline-precompile';
import cleanWhitespace from '../../utils/clean-whitespace';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | placement failures', function (hooks) {
  setupRenderingTest(hooks);

  const commonTemplate = hbs`
      <PlacementFailure @taskGroup={{taskGroup}} />
  `;

  test('should render the placement failure (basic render)', async function (assert) {
    assert.expect(12);

    const name = 'Placement Failure';
    const failures = 11;
    this.set(
      'taskGroup',
      createFixture(
        {
          coalescedFailures: failures - 1,
        },
        name
      )
    );

    await render(commonTemplate);

    assert.equal(
      cleanWhitespace(
        find('[data-test-placement-failure-task-group]').firstChild.wholeText
      ),
      name,
      'Title is rendered with the name of the placement failure'
    );
    assert.equal(
      parseInt(
        find('[data-test-placement-failure-coalesced-failures]').textContent
      ),
      failures,
      'Title is rendered correctly with a count of unplaced'
    );
    assert.equal(
      findAll('[data-test-placement-failure-no-evaluated-nodes]').length,
      1,
      'No evaluated nodes message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-no-nodes-available]').length,
      1,
      'No nodes in datacenter message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-class-filtered]').length,
      1,
      'Class filtered message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-constraint-filtered]').length,
      1,
      'Constraint filtered message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-nodes-exhausted]').length,
      1,
      'Node exhausted message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-class-exhausted]').length,
      1,
      'Class exhausted message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-dimension-exhausted]').length,
      1,
      'Dimension exhausted message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-quota-exhausted]').length,
      1,
      'Quota exhausted message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-scores]').length,
      1,
      'Scores message shown'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('should render correctly when a node is not evaluated', async function (assert) {
    assert.expect(3);

    this.set(
      'taskGroup',
      createFixture({
        nodesEvaluated: 1,
        nodesExhausted: 0,
      })
    );

    await render(commonTemplate);

    assert.equal(
      findAll('[data-test-placement-failure-no-evaluated-nodes]').length,
      0,
      'No evaluated nodes message shown'
    );
    assert.equal(
      findAll('[data-test-placement-failure-nodes-exhausted]').length,
      0,
      'Nodes exhausted message NOT shown when there are no nodes exhausted'
    );

    await componentA11yAudit(this.element, assert);
  });

  function createFixture(obj = {}, name = 'Placement Failure') {
    return {
      name: name,
      placementFailures: assign(
        {
          name: name,
          coalescedFailures: 10,
          nodesEvaluated: 0,
          nodesAvailable: {
            datacenter: 0,
          },
          classFiltered: {
            filtered: 1,
          },
          constraintFiltered: {
            'prop = val': 1,
          },
          nodesExhausted: 3,
          classExhausted: {
            class: 3,
          },
          dimensionExhausted: {
            iops: 3,
          },
          quotaExhausted: {
            quota: 'dimension',
          },
          scores: {
            name: 3,
          },
        },
        obj
      ),
    };
  }
});
