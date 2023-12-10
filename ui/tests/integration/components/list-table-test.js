/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { findAll, find, render } from '@ember/test-helpers';
import { module, skip, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import faker from 'nomad-ui/mirage/faker';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | list table', function (hooks) {
  setupRenderingTest(hooks);

  const commonTable = Array(10)
    .fill(null)
    .map(() => ({
      firstName: faker.name.firstName(),
      lastName: faker.name.lastName(),
      age: faker.random.number({ min: 18, max: 60 }),
    }));

  // thead
  test('component exposes a thead contextual component', async function (assert) {
    this.set('source', commonTable);
    await render(hbs`
      <ListTable @source={{source}} @sortProperty={{sortProperty}} @sortDescending={{sortDescending}} as |t|>
        <t.head @class="head">
          <th>First Name</th>
          <th>Last Name</th>
          <th>Age</th>
        </t.head>
      </ListTable>
    `);

    assert.ok(findAll('.head').length, 'Table head is rendered');
    assert.equal(
      find('.head').tagName.toLowerCase(),
      'thead',
      'Table head is a thead element'
    );
  });

  // tbody
  test('component exposes a tbody contextual component', async function (assert) {
    assert.expect(44);

    this.setProperties({
      source: commonTable,
      sortProperty: 'firstName',
      sortDescending: false,
    });
    await render(hbs`
      <ListTable @source={{source}} @sortProperty={{sortProperty}} @sortDescending={{sortDescending}} as |t|>
        <t.body @class="body" as |row index|>
          <tr class="item">
            <td>{{row.model.firstName}}</td>
            <td>{{row.model.lastName}}</td>
            <td>{{row.model.age}}</td>
            <td>{{index}}</td>
          </tr>
        </t.body>
      </ListTable>
    `);

    assert.ok(findAll('.body').length, 'Table body is rendered');
    assert.equal(
      find('.body').tagName.toLowerCase(),
      'tbody',
      'Table body is a tbody element'
    );

    assert.equal(
      findAll('.item').length,
      this.get('source.length'),
      'Each item gets its own row'
    );

    // list-table is not responsible for sorting, only dispatching sort events. The table is still
    // rendered in index-order.
    this.source.forEach((item, index) => {
      const $item = this.element.querySelectorAll('.item')[index];
      assert.equal(
        $item.querySelectorAll('td')[0].innerHTML.trim(),
        item.firstName,
        'First name'
      );
      assert.equal(
        $item.querySelectorAll('td')[1].innerHTML.trim(),
        item.lastName,
        'Last name'
      );
      assert.equal(
        $item.querySelectorAll('td')[2].innerHTML.trim(),
        item.age,
        'Age'
      );
      assert.equal(
        $item.querySelectorAll('td')[3].innerHTML.trim(),
        index,
        'Index'
      );
    });

    await componentA11yAudit(this.element, assert);
  });

  // Ember doesn't support query params (or controllers or routes) in integration tests,
  // so sorting links can only be tested in acceptance tests.
  // Leaving this test here for posterity.
  skip('sort-by creates links using the appropriate links given sort property and sort descending', function () {});
});
