/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { ServerError, ForbiddenError } from '@ember-data/adapter/error';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

const testCases = [
  {
    name: 'Forbidden Error',
    in: [new ForbiddenError([], "Can't do that"), 'run tests'],
    out: 'Your ACL token does not grant permission to run tests.',
  },
  {
    name: 'Generic Error',
    in: [
      new ServerError([{ detail: 'DB Max Connections' }], 'Server Error'),
      'run tests',
    ],
    out: 'DB Max Connections',
  },
  {
    name: 'Multiple Errors',
    in: [
      new ServerError(
        [{ detail: 'DB Max Connections' }, { detail: 'Service timeout' }],
        'Server Error'
      ),
      'run tests',
    ],
    out: 'DB Max Connections\n\nService timeout',
  },
  {
    name: 'Malformed Error (not from Ember Data which should always have an errors list)',
    in: [new Error('Go boom'), 'handle custom error messages'],
    out: 'Unknown Error',
  },
];

module('Unit | Util | messageFromAdapterError', function () {
  testCases.forEach((testCase) => {
    test(testCase.name, function (assert) {
      assert.equal(
        messageFromAdapterError.apply(null, testCase.in),
        testCase.out,
        `[${testCase.in.join(', ')}] => ${testCase.out}`
      );
    });
  });
});
