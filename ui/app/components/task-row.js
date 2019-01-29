import Ember from 'ember';
import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { task, timeout } from 'ember-concurrency';
import { lazyClick } from '../helpers/lazy-click';

export default Component.extend({
  store: service(),
  token: service(),
  statsTrackersRegistry: service('stats-trackers-registry'),

  tagName: 'tr',
  classNames: ['task-row', 'is-interactive'],

  task: null,

  // Internal state
  statsError: false,

  enablePolling: computed(() => !Ember.testing),

  // Since all tasks for an allocation share the same tracker, use the registry
  stats: computed('task', 'task.isRunning', function() {
    if (!this.get('task.isRunning')) return;

    return this.get('statsTrackersRegistry').getTracker(this.get('task.allocation'));
  }),

  taskStats: computed('task.name', 'stats.tasks.[]', function() {
    if (!this.get('stats')) return;

    return this.get('stats.tasks').findBy('task', this.get('task.name'));
  }),

  cpu: alias('taskStats.cpu.lastObject'),
  memory: alias('taskStats.memory.lastObject'),

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },

  fetchStats: task(function*() {
    do {
      if (this.get('stats')) {
        try {
          yield this.get('stats.poll').perform();
          this.set('statsError', false);
        } catch (error) {
          this.set('statsError', true);
        }
      }

      yield timeout(500);
    } while (this.get('enablePolling'));
  }).drop(),

  didReceiveAttrs() {
    const allocation = this.get('task.allocation');

    if (allocation) {
      this.get('fetchStats').perform();
    } else {
      this.get('fetchStats').cancelAll();
    }
  },
});
