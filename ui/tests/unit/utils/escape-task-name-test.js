/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import escapeTaskName from 'nomad-ui/utils/escape-task-name';
import { module, test } from 'qunit';

module('Unit | Utility | escape-task-name', function () {
  test('it escapes task names for the faux exec CLI', function (assert) {
    assert.equal(escapeTaskName('plain'), 'plain');
    assert.equal(escapeTaskName('a space'), 'a\\ space');
    assert.equal(escapeTaskName('dollar $ign'), 'dollar\\ \\$ign');
    assert.equal(escapeTaskName('emoji🥳'), 'emoji\\🥳');
  });
});
