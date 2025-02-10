/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, findAll, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import PathTree from 'nomad-ui/utils/path-tree';

module('Integration | Component | attributes table', function (hooks) {
  setupRenderingTest(hooks);

  const commonAttributes = [
    {
      path: 'key',
      value: 'value',
    },
    {
      path: 'nested.props',
      value: 'are',
    },
    {
      path: 'nested.supported',
      value: 'just',
    },
    {
      path: 'nested.fine',
      value: null,
    },
    {
      path: 'so.are.deeply.nested',
      value: 'properties',
    },
    {
      path: 'so.are.deeply.like',
      value: 'these ones',
    },
  ];

  const commonAttributesTree = new PathTree(commonAttributes, {
    delimiter: '.',
  });

  test('should render a row for each key/value pair in a deep object', async function (assert) {
    assert.expect(2);

    this.set('attributes', commonAttributesTree.root);
    await render(hbs`<AttributesTable @attributePairs={{attributes}} />`);

    const rowsCount = commonAttributes.length;
    assert.equal(
      this.element.querySelectorAll(
        '[data-test-attributes-section] [data-test-value]'
      ).length,
      rowsCount,
      `Table has ${rowsCount} rows with values`
    );

    await componentA11yAudit(this.element, assert);
  });

  test('should render the full path of key/value pair from the root of the object', async function (assert) {
    this.set('attributes', commonAttributesTree.root);
    await render(hbs`<AttributesTable @attributePairs={{attributes}} />`);

    assert.equal(
      find('[data-test-key]').textContent.trim(),
      'key',
      'Row renders the key'
    );
    assert.equal(
      find('[data-test-value]').textContent.trim(),
      'value',
      'Row renders the value'
    );
    const deepRow = findAll('[data-test-attributes-section]')[4];
    assert.equal(
      deepRow.querySelector('[data-test-key]').textContent.trim(),
      'so.are.deeply.nested',
      'Complex row renders the full path to the key'
    );
    assert.equal(
      deepRow.querySelector('[data-test-prefix]').textContent.trim(),
      'so.are.deeply.',
      'The prefix is faded to put emphasis on the attribute'
    );
    assert.equal(
      deepRow.querySelector('[data-test-value]').textContent.trim(),
      'properties'
    );
  });
});
