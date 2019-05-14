import Ember from 'ember';
import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { task, timeout } from 'ember-concurrency';

export default Component.extend({
  token: service(),
  statsTrackersRegistry: service('stats-trackers-registry'),

  classNames: ['primary-metric'],

  // One of Node, Allocation, or TaskState
  resource: null,

  // cpu or memory
  metric: null,

  'data-test-primary-metric': true,

  // An instance of a StatsTracker. An alternative interface to resource
  tracker: computed('trackedResource', 'type', function() {
    const resource = this.trackedResource;
    return this.statsTrackersRegistry.getTracker(resource);
  }),

  type: computed('resource', function() {
    const resource = this.resource;
    return resource && resource.constructor.modelName;
  }),

  trackedResource: computed('resource', 'type', function() {
    // TaskStates use the allocation stats tracker
    return this.type === 'task-state'
      ? this.get('resource.allocation')
      : this.resource;
  }),

  metricLabel: computed('metric', function() {
    const metric = this.metric;
    const mappings = {
      cpu: 'CPU',
      memory: 'Memory',
    };
    return mappings[metric] || metric;
  }),

  data: computed('resource', 'metric', 'type', function() {
    if (!this.tracker) return [];

    const metric = this.metric;
    if (this.type === 'task-state') {
      // handle getting the right task out of the tracker
      const task = this.get('tracker.tasks').findBy('task', this.get('resource.name'));
      return task && task[metric];
    }

    return this.get(`tracker.${metric}`);
  }),

  reservedAmount: computed('resource', 'metric', 'type', function() {
    const metricProperty = this.metric === 'cpu' ? 'reservedCPU' : 'reservedMemory';

    if (this.type === 'task-state') {
      const task = this.get('tracker.tasks').findBy('task', this.get('resource.name'));
      return task[metricProperty];
    }

    return this.get(`tracker.${metricProperty}`);
  }),

  chartClass: computed('metric', function() {
    const metric = this.metric;
    const mappings = {
      cpu: 'is-info',
      memory: 'is-danger',
    };

    return mappings[metric] || 'is-primary';
  }),

  poller: task(function*() {
    do {
      this.get('tracker.poll').perform();
      yield timeout(100);
    } while (!Ember.testing);
  }),

  didReceiveAttrs() {
    if (this.tracker) {
      this.poller.perform();
    }
  },

  willDestroy() {
    this.poller.cancelAll();
    this.get('tracker.signalPause').perform();
  },
});
