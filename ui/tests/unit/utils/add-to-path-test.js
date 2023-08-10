/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import addToPath from 'nomad-ui/utils/add-to-path';

const testCases = [
  {
    name: 'Only domain',
    in: ['https://domain.com', '/path'],
    out: 'https://domain.com/path',
  },
  {
    name: 'Deep path',
    in: ['https://domain.com/a/path', '/to/nowhere'],
    out: 'https://domain.com/a/path/to/nowhere',
  },
  {
    name: 'With Query Params',
    in: ['https://domain.com?interesting=development', '/this-is-an'],
    out: 'https://domain.com/this-is-an?interesting=development',
  },
];

module('Unit | Util | addToPath', function () {
  testCases.forEach((testCase) => {
    test(testCase.name, function (assert) {
      assert.equal(
        addToPath.apply(null, testCase.in),
        testCase.out,
        `[${testCase.in.join(', ')}] => ${testCase.out}`
      );
    });
  });
});
