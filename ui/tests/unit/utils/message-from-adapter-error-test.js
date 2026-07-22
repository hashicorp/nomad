/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { ServerError, ForbiddenError } from '@ember-data/adapter/error';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

const testCases = [
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
        'Server Error',
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

module('Unit | Util | messageFromAdapterError', function (hooks) {
  let originalToken;

  hooks.beforeEach(function () {
    originalToken = window.localStorage.nomadTokenSecret;
  });

  hooks.afterEach(function () {
    if (originalToken == null) {
      window.localStorage.removeItem('nomadTokenSecret');
    } else {
      window.localStorage.nomadTokenSecret = originalToken;
    }
  });

  test('Forbidden Error - not signed in', function (assert) {
    window.localStorage.removeItem('nomadTokenSecret');

    assert.deepEqual(
      messageFromAdapterError(
        new ForbiddenError([], "Can't do that"),
        'run tests',
      ),
      'You are not signed in. Please sign in to perform this action.',
      'Returns sign-in guidance when no token is present',
    );
  });

  test('Forbidden Error - signed in, insufficient permissions', function (assert) {
    window.localStorage.nomadTokenSecret = 'some-token';

    assert.deepEqual(
      messageFromAdapterError(
        new ForbiddenError([], "Can't do that"),
        'run tests',
      ),
      'Your ACL token does not grant permission to run tests.',
      'Returns permission message when token is present',
    );
  });

  testCases.forEach((testCase) => {
    test(testCase.name, function (assert) {
      assert.deepEqual(
        messageFromAdapterError.apply(null, testCase.in),
        testCase.out,
        `[${testCase.in.join(', ')}] => ${testCase.out}`,
      );
    });
  });
});
