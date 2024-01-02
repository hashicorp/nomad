/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { isArray } from '@ember/array';
import { module, test } from 'qunit';
import RollingArray from 'nomad-ui/utils/classes/rolling-array';

module('Unit | Util | RollingArray', function () {
  test('has a maxLength property that gets set in the constructor', function (assert) {
    const array = RollingArray(10, 'a', 'b', 'c');
    assert.equal(array.maxLength, 10, 'maxLength is set in the constructor');
    assert.deepEqual(
      array,
      ['a', 'b', 'c'],
      'additional arguments to the constructor become elements'
    );
  });

  test('push works like Array#push', function (assert) {
    const array = RollingArray(10);
    const pushReturn = array.push('a');
    assert.equal(
      pushReturn,
      array.length,
      'the return value from push is equal to the return value of Array#push'
    );
    assert.equal(
      array[0],
      'a',
      'the arguments passed to push are appended to the array'
    );

    array.push('b', 'c', 'd');
    assert.deepEqual(
      array,
      ['a', 'b', 'c', 'd'],
      'the elements already in the array are left in tact and new elements are appended'
    );
  });

  test('when pushing past maxLength, items are removed from the head of the array', function (assert) {
    const array = RollingArray(3);
    const pushReturn = array.push(1, 2, 3, 4);
    assert.deepEqual(
      array,
      [2, 3, 4],
      'The first argument to push is not in the array, but the following three are'
    );
    assert.equal(
      pushReturn,
      array.length,
      'The return value of push is still the array length despite more arguments than possible were provided to push'
    );
  });

  test('when splicing past maxLength, items are removed from the head of the array', function (assert) {
    const array = RollingArray(3, 'a', 'b', 'c');

    array.splice(1, 0, 'z');
    assert.deepEqual(
      array,
      ['z', 'b', 'c'],
      'The new element is inserted as the second element in the array and the first element is removed due to maxLength restrictions'
    );

    array.splice(0, 0, 'pickme');
    assert.deepEqual(
      array,
      ['z', 'b', 'c'],
      'The new element never makes it into the array since it was added at the head of the array and immediately removed'
    );

    array.splice(0, 1, 'pickme');
    assert.deepEqual(
      array,
      ['pickme', 'b', 'c'],
      'The new element makes it into the array since the previous element at the head of the array is first removed due to the second argument to splice'
    );
  });

  test('unshift throws instead of prepending elements', function (assert) {
    const array = RollingArray(5);

    assert.throws(
      () => {
        array.unshift(1);
      },
      /Cannot unshift/,
      'unshift is not supported, but is not undefined'
    );
  });

  test('RollingArray is an instance of Array', function (assert) {
    const array = RollingArray(5);
    assert.strictEqual(array.constructor, Array, 'The constructor is Array');
    assert.ok(array instanceof Array, 'The instanceof check is true');
    assert.ok(isArray(array), 'The ember isArray helper works');
  });
});
