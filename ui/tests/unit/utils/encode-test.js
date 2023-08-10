/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { base64DecodeString, base64EncodeString } from 'nomad-ui/utils/encode';
import { module, test } from 'qunit';

module('Unit | Utility | encode', function () {
  test('it encodes a null input', function (assert) {
    const encoded = base64EncodeString(null);
    const decoded = base64DecodeString(encoded);

    assert.equal(decoded, '');
  });

  test('it encodes an empty string', function (assert) {
    const input = '';
    const encoded = base64EncodeString(input);
    const decoded = base64DecodeString(encoded);

    assert.equal(decoded, input);
  });

  test('it decodes a null input', function (assert) {
    const decoded = base64DecodeString(null);

    assert.equal(decoded, '');
  });

  test('it decodes an empty string', function (assert) {
    const decoded = base64DecodeString('');

    assert.equal(decoded, '');
  });

  test('it encodes and decodes non-ascii with base64', function (assert) {
    const input = 'hello 🥳';
    const encoded = base64EncodeString(input);
    const decoded = base64DecodeString(encoded);

    assert.equal(decoded, input);
  });
});
