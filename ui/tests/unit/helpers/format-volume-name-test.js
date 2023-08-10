/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { formatVolumeName } from 'nomad-ui/helpers/format-volume-name';

module('Unit | Helper | formatVolumeName', function () {
  test('Returns source as string when isPerAlloc is false', function (assert) {
    const expectation = 'my-volume-source';
    assert.equal(
      formatVolumeName(null, {
        source: 'my-volume-source',
        isPerAlloc: false,
        volumeExtension: '[arbitrary]',
      }),
      expectation,
      'false perAlloc'
    );
    assert.equal(
      formatVolumeName(null, {
        source: 'my-volume-source',
        isPerAlloc: null,
        volumeExtension: '[arbitrary]',
      }),
      expectation,
      'null perAlloc'
    );
  });

  test('Returns concatonated name when isPerAlloc is true', function (assert) {
    const expectation = 'my-volume-source[1]';
    assert.equal(
      formatVolumeName(null, {
        source: 'my-volume-source',
        isPerAlloc: true,
        volumeExtension: '[1]',
      }),
      expectation,
      expectation
    );
  });
});
