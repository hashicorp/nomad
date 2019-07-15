import EmberObject, { computed } from '@ember/object';
import Service from '@ember/service';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { find } from '@ember/test-helpers';
import { task } from 'ember-concurrency';
import sinon from 'sinon';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

module('Integration | Component | primary metric', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node');

    const getTrackerSpy = (this.getTrackerSpy = sinon.spy());
    const trackerPollSpy = (this.trackerPollSpy = sinon.spy());
    const trackerSignalPauseSpy = (this.trackerSignalPauseSpy = sinon.spy());

    const MockTracker = EmberObject.extend({
      poll: task(function*() {
        yield trackerPollSpy();
      }),
      signalPause: task(function*() {
        yield trackerSignalPauseSpy();
      }),

      cpu: computed(() => []),
      memory: computed(() => []),
    });

    const mockStatsTrackersRegistry = Service.extend({
      getTracker(...args) {
        getTrackerSpy(...args);
        return MockTracker.create();
      },
    });

    this.owner.register('service:stats-trackers-registry', mockStatsTrackersRegistry);
    this.statsTrackersRegistry = this.owner.lookup('service:stats-trackers-registry');
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  const commonTemplate = hbs`
    {{primary-metric
      resource=resource
      metric=metric}}
  `;

  test('Contains a line chart, a percentage bar, a percentage figure, and an absolute usage figure', function(assert) {
    let resource;
    const metric = 'cpu';

    this.store.findAll('node');

    return settled()
      .then(async () => {
        resource = this.store.peekAll('node').get('firstObject');
        this.setProperties({ resource, metric });

        await render(commonTemplate);
        return settled();
      })
      .then(() => {
        assert.ok(find('[data-test-line-chart]'), 'Line chart');
        assert.ok(find('[data-test-percentage-bar]'), 'Percentage bar');
        assert.ok(find('[data-test-percentage]'), 'Percentage figure');
        assert.ok(find('[data-test-absolute-value]'), 'Absolute usage figure');
      });
  });

  test('The CPU metric maps to is-info', function(assert) {
    let resource;
    const metric = 'cpu';

    this.store.findAll('node');

    return settled()
      .then(async () => {
        resource = this.store.peekAll('node').get('firstObject');
        this.setProperties({ resource, metric });

        await render(commonTemplate);
        return settled();
      })
      .then(() => {
        assert.ok(
          find('[data-test-line-chart] .canvas').classList.contains('is-info'),
          'Info class for CPU metric'
        );
      });
  });

  test('The Memory metric maps to is-danger', function(assert) {
    let resource;
    const metric = 'memory';

    this.store.findAll('node');

    return settled()
      .then(async () => {
        resource = this.store.peekAll('node').get('firstObject');
        this.setProperties({ resource, metric });

        await render(commonTemplate);
        return settled();
      })
      .then(() => {
        assert.ok(
          find('[data-test-line-chart] .canvas').classList.contains('is-danger'),
          'Danger class for Memory metric'
        );
      });
  });

  test('Gets the tracker from the tracker registry', function(assert) {
    let resource;
    const metric = 'cpu';

    this.store.findAll('node');

    return settled()
      .then(async () => {
        resource = this.store.peekAll('node').get('firstObject');
        this.setProperties({ resource, metric });

        await render(commonTemplate);
        return settled();
      })
      .then(() => {
        assert.ok(
          this.getTrackerSpy.calledWith(resource),
          'Uses the tracker registry to get the tracker for the provided resource'
        );
      });
  });

  test('Immediately polls the tracker', function(assert) {
    let resource;
    const metric = 'cpu';

    this.store.findAll('node');

    return settled()
      .then(async () => {
        resource = this.store.peekAll('node').get('firstObject');
        this.setProperties({ resource, metric });

        await render(commonTemplate);
        return settled();
      })
      .then(() => {
        assert.ok(this.trackerPollSpy.calledOnce, 'The tracker is polled immediately');
      });
  });

  test('A pause signal is sent to the tracker when the component is destroyed', function(assert) {
    let resource;
    const metric = 'cpu';

    // Capture a reference to the spy before the component is destroyed
    const trackerSignalPauseSpy = this.trackerSignalPauseSpy;

    this.store.findAll('node');

    return settled()
      .then(async () => {
        resource = this.store.peekAll('node').get('firstObject');
        this.setProperties({ resource, metric, showComponent: true });
        await render(hbs`
          {{#if showComponent}}
            {{primary-metric
              resource=resource
              metric=metric}}
            }}
          {{/if}}
        `);
        return settled();
      })
      .then(() => {
        assert.notOk(trackerSignalPauseSpy.called, 'No pause signal has been sent yet');
        // This will toggle the if statement, resulting the primary-metric component being destroyed.
        this.set('showComponent', false);
        return settled();
      })
      .then(() => {
        assert.ok(trackerSignalPauseSpy.calledOnce, 'A pause signal is sent to the tracker');
      });
  });
});
