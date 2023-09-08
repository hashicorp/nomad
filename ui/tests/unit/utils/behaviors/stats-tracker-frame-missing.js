/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { resolve } from 'rsvp';
import { test } from 'qunit';
import sinon from 'sinon';
import { settled } from '@ember/test-helpers';

const MockResponse = (json) => ({
  ok: true,
  json() {
    return resolve(json);
  },
});

export default function statsTrackerFrameMissing({
  resourceName,
  TrackerConstructor,
  ResourceConstructor,
  mockFrame,
  compileResources,
}) {
  test('a bad response from a fetch request is handled gracefully', async function (assert) {
    const frame = mockFrame(1);
    const [compiledCPU, compiledMemory] = compileResources(frame);

    let shouldFail = false;
    const fetch = () => {
      return resolve(shouldFail ? { ok: false } : MockResponse(frame));
    };

    const resource = ResourceConstructor();
    const tracker = TrackerConstructor.create({
      fetch,
      [resourceName]: resource,
    });

    tracker.get('poll').perform();
    await settled();

    assert.deepEqual(tracker.get('cpu'), [compiledCPU], 'One frame of cpu');
    assert.deepEqual(
      tracker.get('memory'),
      [compiledMemory],
      'One frame of memory'
    );

    shouldFail = true;
    tracker.get('poll').perform();
    await settled();

    assert.deepEqual(
      tracker.get('cpu'),
      [compiledCPU],
      'Still one frame of cpu'
    );
    assert.deepEqual(
      tracker.get('memory'),
      [compiledMemory],
      'Still one frame of memory'
    );
    assert.equal(tracker.get('frameMisses'), 1, 'Frame miss is tracked');

    shouldFail = false;
    tracker.get('poll').perform();
    await settled();

    assert.deepEqual(
      tracker.get('cpu'),
      [compiledCPU, compiledCPU],
      'Still one frame of cpu'
    );
    assert.deepEqual(
      tracker.get('memory'),
      [compiledMemory, compiledMemory],
      'Still one frame of memory'
    );
    assert.equal(tracker.get('frameMisses'), 0, 'Frame misses is reset');
  });

  test('enough bad responses from fetch consecutively (as set by maxFrameMisses) results in a pause', async function (assert) {
    const fetch = () => {
      return resolve({ ok: false });
    };

    const resource = ResourceConstructor();
    const tracker = TrackerConstructor.create({
      fetch,
      [resourceName]: resource,
      maxFrameMisses: 3,
      pause: sinon.spy(),
    });

    tracker.get('poll').perform();
    await settled();

    assert.equal(tracker.get('frameMisses'), 1, 'Tick misses');
    assert.notOk(tracker.pause.called, 'Pause not called yet');

    tracker.get('poll').perform();
    await settled();

    assert.equal(tracker.get('frameMisses'), 2, 'Tick misses');
    assert.notOk(tracker.pause.called, 'Pause still not called yet');

    tracker.get('poll').perform();
    await settled();

    assert.equal(tracker.get('frameMisses'), 0, 'Misses reset');
    assert.ok(
      tracker.pause.called,
      'Pause called now that frameMisses == maxFrameMisses'
    );
  });
}
