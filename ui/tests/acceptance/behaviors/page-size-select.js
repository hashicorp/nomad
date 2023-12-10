/**
 * Copyright (c) HashiCorp, Inc.
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
    resourceName
  )} is equal to the localStorage user setting for page size`, async function (assert) {
    const storedPageSize = 10;
    window.localStorage.nomadPageSize = storedPageSize;

    await setup.call(this);

    assert.equal(pageObjectList.length, storedPageSize);
    assert.equal(pageObject.pageSizeSelect.selectedOption, storedPageSize);
  });

  test('when the page size user setting is unset, the default page size is 25', async function (assert) {
    await setup.call(this);

    assert.equal(pageObjectList.length, pageObject.pageSize);
    assert.equal(pageObject.pageSizeSelect.selectedOption, pageObject.pageSize);
  });

  test(`changing the page size updates the ${pluralize(
    resourceName
  )} list and also updates the user setting in localStorage`, async function (assert) {
    const desiredPageSize = 10;

    await setup.call(this);

    assert.equal(window.localStorage.nomadPageSize, null);
    assert.equal(pageObjectList.length, pageObject.pageSize);
    assert.equal(pageObject.pageSizeSelect.selectedOption, pageObject.pageSize);

    await selectChoose('[data-test-page-size-select-parent]', desiredPageSize);

    assert.equal(window.localStorage.nomadPageSize, desiredPageSize);
    assert.equal(pageObjectList.length, desiredPageSize);
    assert.equal(pageObject.pageSizeSelect.selectedOption, desiredPageSize);
  });
}
