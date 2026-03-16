/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import formatDuration from 'nomad-ui/utils/format-duration';

module('Unit | Util | formatDuration', function () {
  test('When all units have values, all units are displayed', function (assert) {
    const expectation =
      '39 years 1 month 13 days 23h 31m 30s 987ms 654µs 400ns';
    assert.deepEqual(
      formatDuration(1234567890987654321n),
      expectation,
      expectation,
    );
  });

  test('Any unit without values gets dropped from the display', function (assert) {
    const expectation = '14 days 6h 56m 890ms 980µs';
    assert.deepEqual(
      formatDuration(1234560890980000),
      expectation,
      expectation,
    );
  });

  test('The units option allows for units coarser than nanoseconds', function (assert) {
    const expectation1 = '1s 200ms';
    const expectation2 = '20m';
    const expectation3 = '1 month 1 day';
    assert.deepEqual(formatDuration(1200, 'ms'), expectation1, expectation1);
    assert.deepEqual(formatDuration(1200, 's'), expectation2, expectation2);
    assert.deepEqual(formatDuration(32, 'd'), expectation3, expectation3);
  });

  test('When duration is 0, 0 is shown in terms of the units provided to the function', function (assert) {
    assert.deepEqual(formatDuration(0), '0ns', 'formatDuration(0) -> 0ns');
    assert.deepEqual(
      formatDuration(0, 'year'),
      '0 years',
      'formatDuration(0, "year") -> 0 years',
    );
  });

  test('The longForm option expands suffixes to words', function (assert) {
    const expectation1 = '3 seconds 20ms';
    const expectation2 = '5 hours 59 minutes';
    assert.deepEqual(
      formatDuration(3020, 'ms', true),
      expectation1,
      expectation1,
    );
    assert.deepEqual(
      formatDuration(60 * 5 + 59, 'm', true),
      expectation2,
      expectation2,
    );
  });
});
