/* eslint-disable ember/no-observers */
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { observes } from '@ember-decorators/object';
import { computed as overridable } from 'ember-overridable-computed';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import classic from 'ember-classic-decorator';

@classic
export default class IndexController extends Controller.extend(Sortable) {
  @service token;

  queryParams = [
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  sortProperty = 'name';
  sortDescending = false;

  @alias('model.states') listToSort;
  @alias('listSorted') sortedStates;

  // Set in the route
  preempter = null;

  @overridable(function() {
    // { title, description }
    return null;
  })
  error;

  @computed('model.allocatedResources.ports.@each.label')
  get ports() {
    return (this.get('model.allocatedResources.ports') || []).sortBy('label');
  }

  @computed('model.taskGroup.services.@each.name')
  get services() {
    return this.get('model.taskGroup.services').sortBy('name');
  }

  onDismiss() {
    this.set('error', null);
  }

  @watchRecord('allocation') watchNext;

  @observes('model.nextAllocation.clientStatus')
  observeWatchNext() {
    const nextAllocation = this.model.nextAllocation;
    if (nextAllocation && nextAllocation.content) {
      this.watchNext.perform(nextAllocation);
    } else {
      this.watchNext.cancelAll();
    }
  }

  @task(function*() {
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
  })
  stopAllocation;

  @task(function*() {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Allocation',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  })
  restartAllocation;

  @action
  gotoTask(allocation, task) {
    this.transitionToRoute('allocations.allocation.task', task);
  }

  @action
  taskClick(allocation, task, event) {
    lazyClick([() => this.send('gotoTask', allocation, task), event]);
  }
}
