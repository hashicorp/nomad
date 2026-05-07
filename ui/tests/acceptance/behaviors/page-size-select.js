/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pluralize } from 'ember-inflector';
import { test } from 'qunit';
import { selectChoose } from 'ember-power-select/test-support';

export default function pageSizeSelect({
  resourceName,
  pageObject,
  pageObjectList,
  setup,
}) {
  test(`the number of ${pluralize(
    resourceName,
  )} is equal to the localStorage user setting for page size`, async function (assert) {
    const storedPageSize = 10;
    window.localStorage.setItem('nomadPageSize', String(storedPageSize));

    await setup.call(this);

    assert.strictEqual(pageObjectList.length, storedPageSize);
    assert.strictEqual(
      Number(pageObject.pageSizeSelect.selectedOption),
      storedPageSize,
    );
  });

  test('when the page size user setting is unset, the default page size is 25', async function (assert) {
    await setup.call(this);

    assert.strictEqual(pageObjectList.length, pageObject.pageSize);
    assert.strictEqual(
      Number(pageObject.pageSizeSelect.selectedOption),
      pageObject.pageSize,
    );
  });

  test(`changing the page size updates the ${pluralize(
    resourceName,
  )} list and also updates the user setting in localStorage`, async function (assert) {
    const desiredPageSize = 10;

    await setup.call(this);

    assert.strictEqual(window.localStorage.getItem('nomadPageSize'), null);
    assert.strictEqual(pageObjectList.length, pageObject.pageSize);
    assert.strictEqual(
      Number(pageObject.pageSizeSelect.selectedOption),
      pageObject.pageSize,
    );

    await selectChoose('[data-test-page-size-select-parent]', desiredPageSize);

    assert.strictEqual(
      window.localStorage.getItem('nomadPageSize'),
      String(desiredPageSize),
    );
    assert.strictEqual(pageObjectList.length, desiredPageSize);
    assert.strictEqual(
      Number(pageObject.pageSizeSelect.selectedOption),
      desiredPageSize,
    );
  });
}
