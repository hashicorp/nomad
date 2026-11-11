/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import unwrapJob from 'nomad-ui/utils/unwrap-job';

module('Unit | Util | unwrapJob', function () {
  test('unwraps a top-level Job wrapper', function (assert) {
    const wrapped = {
      Job: {
        ID: 'json-ui-repro',
        Name: 'json-ui-repro',
        Datacenters: ['dc1'],
      },
    };

    assert.deepEqual(
      unwrapJob(wrapped),
      {
        ID: 'json-ui-repro',
        Name: 'json-ui-repro',
        Datacenters: ['dc1'],
      },
      'Unwraps top-level Job property',
    );
  });

  test('unwraps a top-level lowercase job wrapper', function (assert) {
    const wrapped = {
      job: {
        ID: 'json-ui-repro',
        Name: 'json-ui-repro',
      },
    };

    assert.deepEqual(
      unwrapJob(wrapped),
      {
        ID: 'json-ui-repro',
        Name: 'json-ui-repro',
      },
      'Unwraps top-level lowercase job property',
    );
  });

  test('handles multiply-nested Job wrappers', function (assert) {
    const wrapped = {
      Job: {
        Job: {
          ID: 'json-ui-repro',
        },
      },
    };

    assert.deepEqual(
      unwrapJob(wrapped),
      {
        ID: 'json-ui-repro',
      },
      'Unwraps multiple layers of Job wrappers',
    );
  });

  test('leaves unwrapped job definitions intact', function (assert) {
    const plain = {
      ID: 'json-ui-repro',
      Name: 'json-ui-repro',
      Datacenters: ['dc1'],
    };

    assert.deepEqual(
      unwrapJob(plain),
      plain,
      'Preserves unwrapped job definition unchanged',
    );
  });

  test('handles null, undefined, primitive, and array values safely', function (assert) {
    assert.strictEqual(unwrapJob(null), null, 'Handles null');
    assert.strictEqual(unwrapJob(undefined), undefined, 'Handles undefined');
    assert.strictEqual(unwrapJob('string'), 'string', 'Handles string');
    assert.strictEqual(unwrapJob(123), 123, 'Handles number');
    assert.deepEqual(unwrapJob([1, 2, 3]), [1, 2, 3], 'Handles array');
  });

  test('handles non-object Job values safely', function (assert) {
    assert.deepEqual(
      unwrapJob({ Job: null }),
      { Job: null },
      'Handles null Job property',
    );
    assert.deepEqual(
      unwrapJob({ Job: 'not-an-object' }),
      { Job: 'not-an-object' },
      'Handles string Job property',
    );
    assert.deepEqual(
      unwrapJob({ Job: [1, 2] }),
      { Job: [1, 2] },
      'Handles array Job property',
    );
  });
});
