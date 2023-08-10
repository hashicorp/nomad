/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject, { computed } from '@ember/object';
import Service from '@ember/service';
import { find, render, clearRender } from '@ember/test-helpers';
import { test } from 'qunit';
import { task } from 'ember-concurrency';
import sinon from 'sinon';

export function setupPrimaryMetricMocks(hooks, tasks = []) {
  hooks.beforeEach(function () {
    const getTrackerSpy = (this.getTrackerSpy = sinon.spy());
    const trackerPollSpy = (this.trackerPollSpy = sinon.spy());
    const trackerSignalPauseSpy = (this.trackerSignalPauseSpy = sinon.spy());

    const MockTracker = EmberObject.extend({
      poll: task(function* () {
        yield trackerPollSpy();
      }),
      signalPause: task(function* () {
        yield trackerSignalPauseSpy();
      }),

      cpu: computed(function () {
        return [];
      }),
      memory: computed(function () {
        return [];
      }),
      tasks,
    });

    const mockStatsTrackersRegistry = Service.extend({
      getTracker(...args) {
        getTrackerSpy(...args);
        return MockTracker.create();
      },
    });

    this.owner.register(
      'service:stats-trackers-registry',
      mockStatsTrackersRegistry
    );
    this.statsTrackersRegistry = this.owner.lookup(
      'service:stats-trackers-registry'
    );
  });
}

export function primaryMetric({ template, findResource, preload }) {
  test('Contains a line chart, a percentage bar, a percentage figure, and an absolute usage figure', async function (assert) {
    const metric = 'cpu';

    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric });

    await render(template);

    assert.ok(find('[data-test-line-chart]'), 'Line chart');
    assert.ok(find('[data-test-percentage-bar]'), 'Percentage bar');
    assert.ok(find('[data-test-percentage]'), 'Percentage figure');
    assert.ok(find('[data-test-absolute-value]'), 'Absolute usage figure');
  });

  test('The CPU metric maps to is-info', async function (assert) {
    const metric = 'cpu';

    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric });

    await render(template);

    assert.ok(
      find('[data-test-current-value]').classList.contains('is-info'),
      'Info class for CPU metric'
    );
  });

  test('The Memory metric maps to is-danger', async function (assert) {
    const metric = 'memory';

    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric });

    await render(template);

    assert.ok(
      find('[data-test-current-value]').classList.contains('is-danger'),
      'Danger class for Memory metric'
    );
  });

  test('Gets the tracker from the tracker registry', async function (assert) {
    const metric = 'cpu';

    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric });

    await render(template);

    const spy =
      this.getTrackerSpy.calledWith(resource) ||
      this.getTrackerSpy.calledWith(resource.allocation);

    assert.ok(
      spy,
      'Uses the tracker registry to get the tracker for the provided resource'
    );
  });

  test('Immediately polls the tracker', async function (assert) {
    const metric = 'cpu';

    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric });

    await render(template);

    assert.ok(
      this.trackerPollSpy.calledOnce,
      'The tracker is polled immediately'
    );
  });

  test('A pause signal is sent to the tracker when the component is destroyed', async function (assert) {
    const metric = 'cpu';

    // Capture a reference to the spy before the component is destroyed
    const trackerSignalPauseSpy = this.trackerSignalPauseSpy;

    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric });
    await render(template);

    assert.notOk(
      trackerSignalPauseSpy.called,
      'No pause signal has been sent yet'
    );
    await clearRender();

    assert.ok(
      trackerSignalPauseSpy.calledOnce,
      'A pause signal is sent to the tracker'
    );
  });
}
