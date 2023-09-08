/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { stringifyObject } from 'nomad-ui/helpers/stringify-object';

const objectToStringify = {
  dog: 'thelonious',
  dogAge: 12,
  dogDegreesHeld: null,
};

module('Unit | Helper | stringify-object', function () {
  test('Contains the correct number of whitespace', function (assert) {
    // assertions with whitespace > 0 are whitespace + 3, to account for quote, newline and \ characters
    assert.equal(
      stringifyObject([objectToStringify], {
        whitespace: 10,
      }).indexOf('dog'),
      13,
      'Ten spaces'
    );
    assert.equal(
      stringifyObject([objectToStringify], {
        whitespace: 5,
      }).indexOf('dog'),
      8,
      'Five spaces'
    );
    assert.equal(
      stringifyObject([objectToStringify], {
        whitespace: 0,
      }).indexOf('dog'),
      2,
      'Zero spaces'
    );
  });
  test('Observes replacer array', function (assert) {
    assert.ok(
      stringifyObject([objectToStringify], {
        replacer: ['dog', 'dogDegreesHeld'],
      }).indexOf('dogDegreesHeld'),
      'Unreplaced value is present'
    );
    assert.equal(
      stringifyObject([objectToStringify], {
        replacer: ['dog', 'dogDegreesHeld'],
      }).indexOf('dogAge'),
      -1,
      'Replaced value is missing'
    );
  });
  test('Observes replacer function', function (assert) {
    assert.ok(
      stringifyObject([objectToStringify], {
        replacer: (k, v) => (v ? v : undefined),
      }).indexOf('dogAge'),
      'Unreplaced value is present'
    );
    assert.equal(
      stringifyObject([objectToStringify], {
        replacer: (k, v) => (v ? v : undefined),
      }).indexOf('dogDegreesHeld'),
      -1,
      'Replaced value is missing'
    );
  });
});
