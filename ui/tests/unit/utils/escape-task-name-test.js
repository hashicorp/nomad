/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import escapeTaskName from 'nomad-ui/utils/escape-task-name';
import { module, test } from 'qunit';

module('Unit | Utility | escape-task-name', function () {
  test('it escapes task names for the faux exec CLI', function (assert) {
    assert.deepEqual(escapeTaskName('plain'), 'plain');
    assert.deepEqual(escapeTaskName('a space'), 'a\\ space');
    assert.deepEqual(escapeTaskName('dollar $ign'), 'dollar\\ \\$ign');
    assert.deepEqual(escapeTaskName('emoji🥳'), 'emoji\\🥳');
  });
});
