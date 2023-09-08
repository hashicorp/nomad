/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Adapter | Variable', function (hooks) {
  setupTest(hooks);

  test('Correctly pluralizes lookups with shortened path', async function (assert) {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('variable');

    let newVariable = await this.store.createRecord('variable');
    // we're incorrectly passing an object with a `Model` interface
    // we should be passing a `Snapshot`
    // hacky fix to rectify the issue
    newVariable.attr = () => {};

    assert.equal(
      this.subject().urlForFindAll('variable'),
      '/v1/vars',
      'pluralizes findAll lookup'
    );
    assert.equal(
      this.subject().urlForFindRecord('foo/bar', 'variable', newVariable),
      `/v1/var/${encodeURIComponent('foo/bar')}?namespace=default`,
      'singularizes findRecord lookup'
    );
  });
});
