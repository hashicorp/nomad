import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed, observer } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { alias, mapBy, sort } from '@ember/object/computed';
import { task } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import { watchRecord } from 'nomad-ui/utils/properties/watch';

export default Controller.extend(Sortable, {
  token: service(),

  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'name',
  sortDescending: false,

  listToSort: alias('model.states'),
  sortedStates: alias('listSorted'),

  stateTasks: mapBy('model.states', 'task'),

  lifecyclePhases: computed('stateTasks.@each.lifecycle', 'model.states.@each.state', function() {
    const lifecycleTaskStateLists = this.get('model.states').reduce(
      (lists, state) => {
        const lifecycle = state.task.lifecycle;

        if (lifecycle) {
          if (lifecycle.sidecar) {
            lists.sidecars.push(state);
          } else {
            lists.prestarts.push(state);
          }
        } else {
          lists.mains.push(state);
        }

        return lists;
      },
      {
        prestarts: [],
        sidecars: [],
        mains: [],
      }
    );

    const phases = [];

    if (lifecycleTaskStateLists.prestarts.length || lifecycleTaskStateLists.sidecars.length) {
      phases.push({
        name: 'PreStart',
        isActive: lifecycleTaskStateLists.prestarts.some(state => state.state === 'running'),
      });
    }

    if (lifecycleTaskStateLists.sidecars.length || lifecycleTaskStateLists.mains.length) {
      phases.push({
        name: 'Main',
        isActive: lifecycleTaskStateLists.mains.some(state => state.state === 'running'),
      });
    }

    return phases;
  }),

  sortedLifecycleTaskStates: sort('model.states', function(a, b) {
    // FIXME sorts prestart, sidecar, main, secondary by name, correct?
    return getTaskSortPrefix(a.task).localeCompare(getTaskSortPrefix(b.task));
  }),

  // Set in the route
  preempter: null,

  error: overridable(() => {
    // { title, description }
    return null;
  }),

  network: alias('model.allocatedResources.networks.firstObject'),

  services: computed('model.taskGroup.services.@each.name', function() {
    return this.get('model.taskGroup.services').sortBy('name');
  }),

  onDismiss() {
    this.set('error', null);
  },

  watchNext: watchRecord('allocation'),

  observeWatchNext: observer('model.nextAllocation.clientStatus', function() {
    const nextAllocation = this.model.nextAllocation;
    if (nextAllocation && nextAllocation.content) {
      this.watchNext.perform(nextAllocation);
    } else {
      this.watchNext.cancelAll();
    }
  }),

  stopAllocation: task(function*() {
    try {
      yield this.model.stop();
      // Eagerly update the allocation clientStatus to avoid flickering
      this.model.set('clientStatus', 'complete');
    } catch (err) {
      this.set('error', {
        title: 'Could Not Stop Allocation',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  }),

  restartAllocation: task(function*() {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Allocation',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  }),

  actions: {
    gotoTask(allocation, task) {
      this.transitionToRoute('allocations.allocation.task', task);
    },

    taskClick(allocation, task, event) {
      lazyClick([() => this.send('gotoTask', allocation, task), event]);
    },
  },
});

function getTaskSortPrefix(task) {
  return `${task.lifecycle ? (task.lifecycle.sidecar ? '1' : '0') : '2'}-${task.name}`;
}
